# Shared Go builders for Crossplane.
#
# buildGoModule mints a separate module-download cache (its `goModules`
# fixed-output derivation) per derivation NAME. Without sharing, the
# ~200-module `go mod download` would therefore be realised once for every
# binary arch, the e2e binary, and every check - and CI would push all those
# byte-identical copies to the binary cache.
#
# To avoid that, we build ONE vendor derivation per Go module (root + apis) and
# inject it into every consumer via overrideAttrs, so each module's deps are
# fetched and cached exactly once.
#
# proxyVendor keeps the module *download cache* (not a vendor/ dir) as the
# hashed artifact. It gives the sandboxed `go generate`/`go test` checks the
# full module graph offline, and - unlike `go mod vendor` - does NOT copy the
# locally-replaced ./apis module into the hash, so vendorHashes.root only
# changes on third-party dependency changes, not on every edit to apis/ source.
{ pkgs, self }:
let
  vendorHashes = import ./vendor-hashes.nix;
  go = pkgs.unstable.go_1_25;

  # If the Nix sandbox isn't available (e.g. Renovate's container), Nix falls back to building
  # without it, and $HOME is set to /homeless-shelter on the real filesystem. Give every Go build
  # its own HOME so we don't ever use /homeless-shelter.
  goHome = ''
    export HOME="$NIX_BUILD_TOP/home"
    mkdir -p "$HOME"
  '';

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
    }).goModules.overrideAttrs
      (o: {
        preConfigure = (o.preConfigure or "") + goHome;
      });

  apisVendor =
    (mkVendor "crossplane-apis" {
      src = "${self}/apis";
      vendorHash = vendorHashes.apis;
    }).goModules.overrideAttrs
      (o: {
        preConfigure = (o.preConfigure or "") + goHome;
      });

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
      (o: {
        goModules = modules;
        preConfigure = (o.preConfigure or "") + goHome;
      });
in
{
  # Exposed so `nix run .#tidy` can rebuild each module's vendor cache to
  # capture a fresh hash.
  inherit rootVendor apisVendor;

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

  # apis-module builder (native host platform).
  buildApis = mkBuild {
    modules = apisVendor;
    vendorHash = vendorHashes.apis;
  };
}
