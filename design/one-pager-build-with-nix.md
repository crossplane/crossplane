# Build with Nix

* Owner: Nic Cope (@negz)
* Reviewers: TBD
* Status: Draft

## Background

Building and releasing Crossplane is pretty complex. Our build system must:

* Generate code (CRDs, protobufs, deepcopy methods, goverter conversions)
* Cross-compile Go binaries for 7 platforms
* Build multi-architecture OCI images
* Package a Helm chart
* Run unit tests and linters
* Run a complex mesh of E2E tests (in a `kind` cluster)
* Publish everything to OCI registries and https://releases.crossplane.io

Two years ago we [migrated from the Make-based [build submodule][build] to
[Earthly][earthly]. Most maintainers disliked working with advanced Make, and the
git submodule workflow added friction to build changes. Builds also weren't
hermetic. The submodule pinned most tools but relied on your system's Go
toolchain, `sed` nuances, etc.

Last year Earthly's maintainers [announced][earthly-dead] they would no longer
actively develop Earthly. There's been one release in the past 18 months, and it
was only to announce the project's transition to maintenance mode. A community
fork called [Earthbuild][earthbuild] exists, but it was created six months ago
and hasn't had a release yet. Betting on it would mean betting on another small,
unproven project.

This leaves us needing to replace Earthly.

It's worth noting the Crossplane ecosystem has had a split-brain build system
since we adopted Earthly. This repository and crossplane-runtime use Earthly,
while all providers still use the build submodule. Any replacement we choose
will need to eventually bridge this gap, or we'll need to maintain two build
systems indefinitely.

## Goals

The goals of this proposal are to:

* Replace Earthly with a stable, actively maintained build system.
* Minimize setup friction for contributors.
* Minimize ongoing maintenance burden of build tooling.
* Ensure local development and CI use the same, reproducible toolchain.
* Provide a foundation that could eventually unify core and provider builds.

## Proposal

I propose we replace Earthly with a [Nix flake][nix-flakes].

Nix is a 21-year-old build system, package manager, and Linux distro. It's
governed by the [NixOS Foundation][nixos-foundation].

https://github.com/NixOS/nixpkgs is one of the most active repos on GitHub. It
currently packages over 120,000 tools. nixpkgs has a stable channel with two
releases a year, in April and November (e.g. nixpkgs-25.04 and nixpkgs-25.11).
It also has an unstable channel that trails the main branch by a few days. In my
experience Nix packages are updated really quickly.

Unlike traditional package managers that install tools globally, Nix installs
them into content addressable store at `/nix/store`. This makes installing a
different Go version as easy as `nix shell nixpkgs#go_1_24`. Run that command
and you're in a shell with Go 1.24 in `$PATH`. `exit` and it's gone.

### How It Works

I spent a few days on a Nix POC. The POC adds a `flake.nix` to the repository.

`flake.nix` (a "Nix flake") is a bit like a Makefile backed by a snapshot of
nixpkgs. At a particular git commit (i.e. a particular `flake.lock` revision)
any reference to `${pkgs.go_1_24}/bin/go` is always guaranteed to reference the
same Go binary. If you don't have the binary locally, Nix will download it.

### Local Development

The simplest way to work on Crossplane is with `nix.sh`, which runs Nix inside
Docker - no installation required:

```sh
./nix.sh run .#test         # Run unit tests
./nix.sh run .#lint         # Run golangci-lint with --fix
./nix.sh run .#generate     # Run code generation
./nix.sh run .#e2e          # Run E2E tests
./nix.sh build              # Build binaries and images

./nix.sh flake show         # See all available commands
```

The first run downloads all dependencies - the Go toolchain, linters, code
dependencies, etc - into a Docker volume (~2-3 min with a fast connection).
Subsequent runs reuse the cache and take seconds. CI pushes to a shared binary
cache, so if it recently built your commit you'll download pre-built artifacts
instead of rebuilding.

Under the hood, `nix.sh` runs a Docker container with:

* No `/nix` directory, daemon, or root access needed on the host
* Automatic flake and binary cache configuration
* Persistent caches (Nix store, Go modules, Docker images) in a volume
* Docker-in-Docker for E2E tests that need kind clusters

The container runs `--privileged` for Docker-in-Docker support. Performance
matches native Nix.

Contributors who want Nix integrated with their shell and dotfiles can install
it natively. This lets you run `nix develop` to get a shell with all tools, or
use direnv for automatic environment activation.

```sh
# Install Nix from https://nixos.org/download/, then:
nix run .#test
nix develop -c $SHELL   # Drop into a shell with Go, kubectl, helm, kind, etc.
```

### CI

CI runs `nix build` and `nix flake check`. Nix runs these in a sandbox without
network or filesystem access. All inputs are content-addressed:

* `flake.lock` pins the exact nixpkgs commit (and thus exact tool versions)
* `gomod2nix.toml` pins hashes of every Go module dependency
* The source is the git commit itself

This means `nix build` on commit N today will produce the same binary as `nix
build` on commit N next year. All inputs are recorded and the build is isolated
from ambient system state. This is useful for supply chain compliance - "what
inputs produced this artifact?" has a precise, verifiable answer.

### Under The Hood

`flake.nix` is a bit like a Makefile backed by a pinned snapshot of nixpkgs. It
defines:

* **Packages**: Cross-compiled Go binaries for all platforms, OCI images, and
  the Helm chart.
* **Checks**: Unit tests, linters, and generated code verification.
* **Apps**: Fast commands for local development (`nix run .#test`, etc).
* **DevShell**: The development environment with all tools pinned.

In Nix, 'pure' essentially means hermetic, whereas 'impure' isn't. Packages and
checks are pure, apps aren't. Packages are built and checks are run in a sandbox
without network or filesystem access. Apps run in your local environment.

Here's what building the crossplane binary looks like:

```nix
crossplane = pkgs.buildGoApplication {
  pname = "crossplane";
  inherit version;
  src = self;
  modules = ./gomod2nix.toml;
  subPackages = [ "cmd/crossplane" ];
  CGO_ENABLED = "0";
  ldflags = [
    "-s" "-w"
    "-X=github.com/crossplane/crossplane/internal/version.version=${version}"
  ];
};
```

And here's the dev shell - just a list of tools we want available:

```nix
devShells.default = pkgs.mkShell {
  buildInputs = [
    pkgs.go_1_24
    pkgs.golangci-lint
    pkgs.kubectl
    pkgs.kubernetes-helm
    pkgs.kind
  ];
};
```

The `flake.lock` file pins the exact nixpkgs commit. Everyone who runs
`nix develop` gets identical tool versions - not just "the same version of
golangci-lint" but the same Go compiler, the same protoc, everything.

### Caching

Nix uses a content-addressable binary cache. Anything it builds can be uploaded
to the cache, so that future builds don't need to rebuild it.

Notably the `buildGoApplication` function builds and caches everything in
`go.mod` as a distinct artifact. So as long as `go.mod` doesn't change CI (and
developers) rarely need to build dependencies. They can just pull them from
cache.

The publish-artifacts job runs in GitHub Actions on every PR. It builds
Crossplane binaries and OCI images for several platforms:


| Build System      | publish-artifacts Time |
|-------------------|------------------------|
| Earthly           | ~20 min                |
| Nix (cold cache)  | ~20 min                |
| Nix (hot cache)   | ~5 min                 |


When Nix has a hot cache all dependencies are pre-compiled, saving ~15 mins of
CI compile time.

Since local development (i.e. `nix develop` or `nix run .#test`) is essentially
an overlay on your regular development environment, it benefits from the typical
Go module cache, build cache, etc.

## Risks

### Learning Curve

Nix has a learning curve. The language is functional and declarative, which can
feel alien to developers used to imperative scripts. It's fair to be concerned
about Nix's learning curve - the alien language was a key reason for moving away
from advanced Make.

A few mitigating factors:

* Nix has become surprisingly popular lately. It's pretty Googleable.
* LLMs like Claude understand Nix well. Much of the POC was LLM-assisted.
* Simple apps and checks are essentially inline shell scripts.

For example, `nix run .#test` is just:

```nix
test = {
  type = "app";
  meta.description = "Run unit tests";
  program = pkgs.lib.getExe (
    pkgs.writeShellScriptBin "test" ''
      set -e
      ${pkgs.go_1_24}/bin/go test -covermode=count ./apis/... ./cmd/... ./internal/... "$@"
    ''
  );
};
```

### Installation Friction

Early testing revealed that installing Nix was a significant barrier. The macOS
installer requires root access to create a synthetic volume at `/nix`, and some
contributors couldn't get it working at all.

We addressed this with `nix.sh`, a wrapper that runs Nix inside Docker. This
makes Docker the only prerequisite - contributors don't need to install Nix
locally. Power users who want native Nix integration with their shell can still
install it, but it's not required.

## Future Improvements

### Cross-Repository Sharing

Looking ahead, Nix flakes can import other flakes without git submodules. If we
eventually migrate providers to Nix, we could create a `crossplane/nix`
repository with shared derivations:

```nix
{
  inputs.crossplane.url = "github:crossplane/nix";
  
  outputs = { self, crossplane, ... }: {
    # Use shared build logic from crossplane/nix
  };
}
```

This would let us share build logic across repos without git submodules.

## Alternatives Considered

### Return to the build submodule

The most obvious alternative is to return to Make and the build submodule. This
would reunify the ecosystem - providers still use it, so we'd have one build
system again.

The build submodule works. It's maintained, stable, and familiar to long-time
contributors. Make itself is 47 years old and isn't going anywhere.

However, the reasons we moved away from the build submodule still apply:

* Advanced Make has a high learning curve. The `$(foreach p,$(GO_STATIC_PACKAGES),...)`
  patterns in `golang.mk` are no easier to read than Nix.
* The git submodule workflow adds friction. Changes require PRs to two repos
  and keeping them in sync.
* Builds aren't reproducible. The submodule pins tool versions but uses your
  system's Go toolchain and global caches. "Works on my machine" remains
  possible.
* We'd still need to maintain tool download targets - the curl/unzip logic for
  each tool we depend on.

Returning to the build submodule would mean going backward to a system we
already decided wasn't good enough. The ecosystem split is a real cost of not
returning, but I believe it's better to move forward and eventually migrate
providers than to move backward.

### Dagger

[Dagger][dagger] is architecturally similar to Earthly - it's built on BuildKit
and provides containerized builds. It's more actively developed than Earthly.

However, Dagger is a commercial open-source project backed by a single vendor.
This is exactly the situation we're in with Earthly. The Dagger team needs to
build a business, and their incentives may not always align with ours. We've
been burned once by betting on commercial open-source build tooling; I'm
hesitant to do it again.

Dagger also requires writing build logic in a general-purpose language like Go
or Python, which is more complex than either a Makefile or a Nix flake for our
use cases.

### Stay on Earthly

This isn't really viable. Earthly is in maintenance mode with no planned feature
development. The community fork (Earthbuild) is too new and unproven to bet on.
We'd be accumulating risk the longer we stay.

[build]: https://github.com/crossplane/build
[earthly]: defunct/one-pager-build-with-earthly.md
[earthly-dead]: https://github.com/earthly/earthly/issues/4313
[nix-flakes]: https://nixos.wiki/wiki/Flakes
[nixos-foundation]: https://nixos.org/community/#foundation
[gomod2nix]: https://github.com/nix-community/gomod2nix
[magic-cache]: https://github.com/DeterminateSystems/magic-nix-cache-action
[dagger]: https://dagger.io
[earthbuild]: https://github.com/earthbuild/earthbuild
