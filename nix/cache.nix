# Per-platform Go build cache for cross-compilation.
#
# This is a workaround for gomod2nix's mkGoCacheEnv only building caches for
# the native platform (it inherits GOOS/GOARCH from the go package).
#
# TODO(negz): Open an upstream PR to add this to gomod2nix.
{
  self,
  pkgs,
  go,
  goPlatforms,
}:
let
  modulesStruct = builtins.fromTOML (builtins.readFile ../gomod2nix.toml);
  cachePackages = modulesStruct.cachePackages or [ ];

  # Filter source to only dependency-related files. This ensures the cache
  # derivation hash only changes when dependencies change (go.mod, go.sum, or
  # gomod2nix.toml), not on every code commit. This is the same approach
  # gomod2nix uses for its depFilesPath.
  depFiles = pkgs.lib.cleanSourceWith {
    src = self;
    filter =
      path: type:
      let
        baseName = baseNameOf path;
      in
      baseName == "go.mod" || baseName == "go.sum" || baseName == "gomod2nix.toml";
    name = "go-dep-files";
  };

  # Get vendor environment from a dummy buildGoApplication's passthru
  # (mkVendorEnv isn't exported by the overlay, but vendorEnv is in passthru)
  vendorEnv =
    (pkgs.buildGoApplication {
      pname = "vendor-env-source";
      version = "0";
      src = self;
      pwd = self;
      modules = ../gomod2nix.toml;
      inherit go;
      dontBuild = true;
      dontCheck = true;
      installPhase = "mkdir $out";
    }).passthru.vendorEnv;

  # Generate the cache.go content
  cacheGoContent = ''
    package main

    import (
    ${builtins.concatStringsSep "\n" (map (pkg: ''	_ "${pkg}"'') cachePackages)}
    )

    func main() {}
  '';

  # Build a Go build cache for a specific GOOS/GOARCH target platform.
  # This pre-compiles all dependencies so cross-compiled builds can reuse them.
  mkGoCacheForPlatform =
    { platform }:
    pkgs.stdenv.mkDerivation {
      name = "go-cache-${platform.os}-${platform.arch}";

      dontUnpack = true;

      nativeBuildInputs = [
        go
        pkgs.rsync
        pkgs.zstd
        pkgs.gnutar
      ];

      # Target platform for cross-compilation cache
      GOOS = platform.os;
      GOARCH = platform.arch;
      CGO_ENABLED = "0";

      configurePhase = ''
        # GO_NO_VENDOR_CHECKS=1 is a nixpkgs patch to Go that bypasses vendor/modules.txt checks
        export GO_NO_VENDOR_CHECKS=1
        export GOCACHE="$TMPDIR/go-cache"
        export GOPATH="$TMPDIR/go"
        export GOSUMDB=off
        export GOPROXY=off
        mkdir -p "$GOCACHE"

        # Set up working directory
        mkdir -p source
        cd source

        # Copy go.mod and go.sum from filtered source (depFiles) so the cache
        # derivation only rebuilds when dependencies change, not on every commit.
        cp ${depFiles}/go.mod ./go.mod
        cp ${depFiles}/go.sum ./go.sum 2>/dev/null || touch go.sum

        # Set up vendor directory
        mkdir -p vendor
        rsync -a ${vendorEnv}/ vendor/
      '';

      buildPhase = ''
        runHook preBuild

        echo "Building cache for ${platform.os}/${platform.arch} (${toString (builtins.length cachePackages)} packages)..."

        # Generate cache.go
        cat > cache.go << 'CACHEGO'
        ${cacheGoContent}
        CACHEGO

        # Build to populate cache (may fail for some packages, that's ok)
        # -trimpath is critical: gomod2nix adds it via GOFLAGS, so the cache must match
        go build -mod=vendor -trimpath -v cache.go || true

        echo "Cache population complete"

        runHook postBuild
      '';

      installPhase = ''
        runHook preInstall

        mkdir -p "$out"
        tar -cf - -C "$GOCACHE" . | zstd -T$NIX_BUILD_CORES -o "$out/cache.tar.zst"

        runHook postInstall
      '';
    };

in
# Return an attrset of caches keyed by "os-arch"
builtins.listToAttrs (
  map (platform: {
    name = "${platform.os}-${platform.arch}";
    value = mkGoCacheForPlatform { inherit platform; };
  }) goPlatforms
)
