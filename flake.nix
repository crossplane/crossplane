# New to Nix? Start here:
#   Language basics:  https://nix.dev/tutorials/nix-language
#   Flakes intro:     https://zero-to-nix.com/concepts/flakes
{
  description = "Crossplane - The cloud native control plane framework";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";

    # TODO(negz): Unpin once https://github.com/nix-community/gomod2nix/pull/231 is released.
    gomod2nix = {
      url = "github:nix-community/gomod2nix/75c2866d585a75a1b30c634dbd7c2dcce5a6c3a7";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      gomod2nix,
    }:
    let
      # Set by CI to override the auto-generated dev version.
      buildVersion = null;

      # Platforms we build Go binaries for.
      goPlatforms = [
        {
          os = "linux";
          arch = "amd64";
        }
        {
          os = "linux";
          arch = "arm64";
        }
        {
          os = "linux";
          arch = "arm";
        }
        {
          os = "linux";
          arch = "ppc64le";
        }
        {
          os = "darwin";
          arch = "arm64";
        }
        {
          os = "darwin";
          arch = "amd64";
        }
        {
          os = "windows";
          arch = "amd64";
        }
      ];

      # Platforms we build OCI images for (Linux only).
      imagePlatforms = builtins.filter (p: p.os == "linux") goPlatforms;

      # Systems where Nix runs (dev machines, CI).
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      # Semantic version for binaries, images, and Helm chart. Uses buildVersion
      # if set by CI, otherwise generates a dev version from git metadata.
      # (self ? shortRev tests if the attribute exists - clean commits have
      # shortRev, uncommitted changes have dirtyShortRev.)
      version =
        if buildVersion != null then
          buildVersion
        else if self ? shortRev then
          "v0.0.0-${builtins.toString self.lastModified}-${self.shortRev}"
        else
          "v0.0.0-${builtins.toString self.lastModified}-${self.dirtyShortRev}";

      # Filtered source for Go builds. Nix will only see these paths when it
      # builds Go code in its sandbox. It starts with 'self' (the repo root),
      # and filters down to *.go, go.mod, etc. Everything Nix needs to see to
      # build Crossplane, build E2E tests, run unit tests, or run golangci-lint
      # must be listed here. The benefit of this filtering is that Nix'll only
      # rebuild Go code when these files change. If you nix build, edit
      # README.md, then nix build again, the second build will be cached.
      src = nixpkgs.lib.sources.cleanSourceWith {
        src = self;
        filter =
          path: type:
          type == "directory"
          || nixpkgs.lib.hasSuffix ".go" path
          || nixpkgs.lib.hasInfix "/testdata/" path
          || builtins.elem (baseNameOf path) [
            "go.mod"
            "go.sum"
            "gomod2nix.toml"
            ".golangci.yml"
          ];
      };

      # Helpers for per-system outputs.
      forAllSystems = f: nixpkgs.lib.genAttrs supportedSystems (system: forSystem system f);
      forSystem =
        system: f:
        f {
          inherit system;
          pkgs = import nixpkgs {
            inherit system;
            overlays = [ gomod2nix.overlays.default ];
          };
        };

    in
    {
      # Build outputs (nix build).
      packages = forAllSystems (
        { pkgs, ... }:
        let
          build = import ./nix/build.nix { inherit pkgs self src; };
        in
        {
          default = build.release {
            inherit
              version
              goPlatforms
              imagePlatforms
              ;
          };
        }
      );

      # CI checks (nix flake check).
      checks = forAllSystems (
        { pkgs, ... }:
        let
          checks = import ./nix/checks.nix { inherit pkgs self src; };
        in
        {
          test = checks.test { inherit version; };
          generate = checks.generate { inherit version; };
          go-lint = checks.goLint { inherit version; };
          helm-lint = checks.helmLint { };
          shell-lint = checks.shellLint { };
          nix-lint = checks.nixLint { };
        }
      );

      # Development commands (nix run .#<app>).
      apps = forAllSystems (
        { pkgs, ... }:
        let
          build = import ./nix/build.nix { inherit pkgs self src; };
          apps = import ./nix/apps.nix { inherit pkgs; };
          nativeArch = if pkgs.stdenv.hostPlatform.isAarch64 then "arm64" else "amd64";
          images = build.images {
            inherit version;
            platforms = imagePlatforms;
          };
        in
        {
          test = apps.test { };
          lint = apps.lint { };
          generate = apps.generate { };
          tidy = apps.tidy { };
          e2e = apps.e2e {
            inherit version;
            inherit (images."linux-${nativeArch}") image;
            bin = build.e2e { inherit version; };
          };
          hack = apps.hack {
            inherit version;
            inherit (images."linux-${nativeArch}") image;
            chart = build.chart { inherit version; };
          };
          unhack = apps.unhack { };
          push-images = apps.pushImages {
            inherit version;
            inherit images;
            platforms = imagePlatforms;
          };
          push-artifacts = apps.pushArtifacts {
            inherit version;
            release = build.release {
              inherit version goPlatforms imagePlatforms;
            };
          };
          promote-images = apps.promoteImages { };
          promote-artifacts = apps.promoteArtifacts { };
          stream-image = apps.streamImage {
            imageArgs = build.imageArgs {
              inherit version;
              arch = nativeArch;
            };
          };
        }
      );

      # Development shell (nix develop).
      devShells = forAllSystems (
        { pkgs, ... }:
        {
          default = pkgs.mkShell {
            buildInputs = [
              pkgs.coreutils
              pkgs.gnused
              pkgs.ncurses
              pkgs.go
              pkgs.golangci-lint
              pkgs.kubectl
              pkgs.kubernetes-helm
              pkgs.kind
              pkgs.docker-client
              pkgs.gotestsum
              pkgs.awscli2
              pkgs.gomod2nix

              # Code generation
              pkgs.buf
              pkgs.helm-docs
              pkgs.goverter
              pkgs.protoc-gen-go
              pkgs.protoc-gen-go-grpc
              pkgs.kubernetes-controller-tools

              # Nix
              pkgs.nixfmt-rfc-style
            ];

            shellHook = ''
              export PS1='\[\033[38;2;243;128;123m\][cros\[\033[38;2;255;205;60m\]spla\[\033[38;2;53;208;186m\]ne]\[\033[0m\] \w \$ '

              source <(kubectl completion bash 2>/dev/null)
              source <(helm completion bash 2>/dev/null)
              source <(kind completion bash 2>/dev/null)

              alias k=kubectl

              echo "Crossplane development shell ($(go version | cut -d' ' -f3))"
              echo ""
              echo "  nix run .#test          nix run .#generate"
              echo "  nix run .#lint          nix run .#tidy"
              echo "  nix run .#e2e           nix run .#hack"
              echo "  nix run .#stream-image | docker load"
              echo ""
              echo "  nix build               nix flake check"
              echo ""
            '';
          };
        }
      );
    };
}
