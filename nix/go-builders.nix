# Shared Go builders for Crossplane.
#
# buildGoModule mints a separate module-download cache (its `goModules`
# fixed-output derivation) per derivation NAME. Without sharing, the
# ~200-module `go mod download` would therefore be realised once for every
# binary arch, the e2e binary, and every check - and CI would push all those
# byte-identical copies to the binary cache.
#
# To avoid that, we build ONE vendor derivation per Go module and inject it
# into every consumer via overrideAttrs, so each module's deps are fetched and
# cached exactly once.
#
# proxyVendor keeps the module *download cache* (not a vendor/ dir) as the
# hashed artifact. It gives the sandboxed `go generate`/`go test` checks the
# full module graph offline.
{ pkgs, self }:
let
  vendorHashes = import ./vendor-hashes.nix;
  go = pkgs.go-unstable;

  # One shared module-download cache per module. Built with the native toolchain
  # (`go mod download` is platform-independent), so every consumer - including
  # cross-compiled binaries - shares the same derivation.
  mkVendor =
    name:
    { src, vendorHash }:
    (pkgs.buildGoModule.override { inherit go; }) {
      pname = name;
      version = "vendor";
      inherit src vendorHash;
      proxyVendor = true;
    };

  rootVendor =
    (mkVendor "crossplane-root" {
      src = self;
      vendorHash = vendorHashes.root;
    }).goModules;

  # Build for a module, injecting that module's shared vendor cache so no
  # per-derivation goModules is built. `goAttrs` lets callers cross-compile by
  # merging GOOS/GOARCH into the go package (buildGoModule reads them from
  # there, not from the build env). CGO is off, so no cross C toolchain is
  # needed.
  mkBuild =
    {
      modules,
      vendorHash,
      goAttrs ? go,
    }:
    args:
    ((pkgs.buildGoModule.override { go = goAttrs; }) (
      {
        inherit vendorHash;
        proxyVendor = true;
      }
      // args
    )).overrideAttrs
      (_: {
        goModules = modules;
      });
in
{
  # Exposed so `nix run .#tidy` can rebuild each module's vendor cache to
  # capture a fresh hash.
  inherit rootVendor;

  # Root-module builder (native host platform).
  buildRoot = mkBuild {
    modules = rootVendor;
    vendorHash = vendorHashes.root;
  };

  # Root-module builder, cross-compiled to a target platform.
  buildRootFor =
    platform:
    mkBuild {
      modules = rootVendor;
      vendorHash = vendorHashes.root;
      goAttrs = go // {
        GOOS = platform.os;
        GOARCH = platform.arch;
      };
    };
}
