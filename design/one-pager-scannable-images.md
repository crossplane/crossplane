# Scannable Container Images

* Owner: Philippe Scorsolini (@phisco)
* Reviewers: Nic Cope (@negz)
* Status: Draft

## Background

Crossplane builds its `crossplane` binary and OCI images entirely with Nix (see
[Build with Nix][build-with-nix]). The Go binary is built with [gomod2nix]'s
`buildGoApplication`.

Running an image vulnerability scanner - [grype] or [trivy] - against a
published `crossplane/crossplane` image reports only operating-system and Go
*stdlib* advisories. It never reports vulnerabilities in Crossplane's
third-party Go dependencies, even when known CVEs exist for the exact versions
we publish.

That is dangerous: a passing scan is a false green signal. The images look
"clean" to anyone who scans them - including downstream security teams and
compliance pipelines - while every third-party Go CVE is invisible.

### Root cause

grype and trivy enumerate an image's Go modules from the dependency list that
the Go toolchain embeds in every binary's build info
(`runtime/debug.BuildInfo`, visible via `go version -m`). Extracting the binary
from a published image and inspecting it shows the problem:

```
$ go version -m crossplane
crossplane: go1.25.x
	path	github.com/crossplane/crossplane/v2/cmd/crossplane
	mod	github.com/crossplane/crossplane/v2	(devel)
	build	-buildmode=exe ... -trimpath=true CGO_ENABLED=0 ...
```

There are **zero `dep` lines** - the entire third-party module list is missing.
A scanner then only sees `go1.25.x` (stdlib) plus the image's OS
packages.

The dependency list is written by the Go toolchain during a normal
module-aware build. This is not caused by `-s -w` or Nix's binary strip: a
nixpkgs `buildGoModule` binary (e.g. `crane`) keeps its full `dep` list through
the same strip. The cause is specific to gomod2nix's `buildGoApplication`: it
builds against a synthesized module cache that doesn't reproduce the module
metadata the toolchain needs, so the toolchain omits every `dep` entry from the
binary's build info.

## Goals

* Make `grype <image>` / `trivy image <image>` detect Crossplane's third-party
  Go dependency vulnerabilities, with no extra steps for us or downstream
  consumers.
* Keep the all-Nix, hermetic, reproducible build introduced in
  [Build with Nix][build-with-nix].
* Keep dependency-bump automation as turnkey as it is today: one command, run
  by Renovate.

## Proposal

Build the binary - and the images, the e2e binary, and the sandboxed checks -
with nixpkgs' [`buildGoModule`][buildgomodule] instead of gomod2nix's
`buildGoApplication`. `buildGoModule` builds in normal module-aware mode and
preserves the dependency list in the binary's build info.

This covers the core `crossplane/crossplane` binary and image; Crossplane
providers have separate build systems and are out of scope.

The effect, measured on the resulting image:

| | gomod2nix (today) | buildGoModule |
|---|---|---|
| `dep` lines in the binary | 0 | ~175 |
| Go modules a scanner catalogs | 2 (main module + stdlib) | ~177, all versioned |
| third-party Go CVEs detected | none | yes |

Key implementation choices:

* **`proxyVendor = true`.** The hashed artifact is the module *download cache*
  populated by `go mod download`, rather than a `vendor/` directory. This gives
  the sandboxed `go test` / `go generate` checks the full module graph offline,
  and - because a `replace` directive pointing at `./apis` has nothing to
  download - the root hash excludes `apis/` source, so apis edits never churn
  it.
* **One shared vendor derivation per module.** `buildGoModule` would otherwise
  fetch the ~200-module download cache once per binary arch and once per check.
  We build a single cache per Go module (root and `apis/`) - with the native
  toolchain, since `go mod download` output is platform-independent - and inject
  it into every consumer, so it is realised once rather than per arch/check.
* **`nix/vendor-hashes.nix` + `nix run .#tidy`.** Vendor hashes are pinned in a
  small checked-in file - the direct replacement for the `gomod2nix.toml`
  manifests - and refreshed by `nix run .#tidy`, which Renovate already runs
  after Go dependency upgrades.

This also removes the `gomod2nix` flake input entirely, trading a niche
community project (pinned to an unreleased commit) for the in-tree, broadly
used `buildGoModule`.

## Tradeoffs

* gomod2nix's incremental dependency *compilation* cache is lost; cold-cache CI
  builds recompile dependencies per arch. Hot-cache (Cachix) builds are
  unaffected.
* Each dependency bump refreshes two aggregate vendor hashes (root and `apis/`)
  rather than a per-module manifest. Integrity is still anchored by `go.sum`:
  the download cache is verified against it during the build.
* Image size is effectively unchanged. The binary grows ~68 KiB (+0.1%) - the
  now-embedded dependency list itself - and the image definition (distroless
  base, layers, CRDs) is untouched, so the published image stays ~22 MB.

## Alternatives considered

* **Publish a signed SBOM attestation** generated from `go.mod` alongside the
  image. Valuable for supply-chain provenance, but it does not fix the common
  `grype <image>` / `trivy image <image>` workflow (the scanner must be pointed
  at the attestation), and a source-derived SBOM over-reports: `go.mod` lists
  test-only and platform-specific modules that may not be linked into the
  final binary. Complementary, not a substitute; it can be layered on later,
  and is more accurate once the binary itself is correct.
* **Fix gomod2nix to stamp the build list.** No supported option exists, and the
  outstanding upstream work the flake is waiting on is unrelated to build info.
* **Build images with [ko].** Produces correct build info, but abandons the
  Nix-based image build we just adopted.

[build-with-nix]: one-pager-build-with-nix.md
[gomod2nix]: https://github.com/nix-community/gomod2nix
[buildgomodule]: https://nixos.org/manual/nixpkgs/stable/#ssec-go-modules
[grype]: https://github.com/anchore/grype
[trivy]: https://github.com/aquasecurity/trivy
[ko]: https://ko.build
