{
  description = "Crossplane - The cloud native control plane framework";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";

    nixpkgs-unstable.url = "github:NixOS/nixpkgs/nixos-unstable";

    flake-utils.url = "github:numtide/flake-utils";

    # Pinned to commit with --with-deps support (not yet released)
    gomod2nix = {
      url = "github:nix-community/gomod2nix/49662a44272806ff785df2990a420edaaca15db4";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      nixpkgs-unstable,
      flake-utils,
      gomod2nix,
    }:
    let
      # Overlays to extend nixpkgs (e.g., access unstable packages via pkgs.unstable.<name>)
      overlays = [
        gomod2nix.overlays.default
        (final: prev: {
          unstable = import nixpkgs-unstable {
            system = prev.stdenv.hostPlatform.system;
          };
        })
      ];

      version =
        if self ? shortRev then
          "v0.0.0-${builtins.toString self.lastModified}-${self.shortRev}"
        else
          "v0.0.0-${builtins.toString self.lastModified}-${self.dirtyShortRev}";

      # Strip leading 'v' for Helm chart version
      chartVersion = builtins.substring 1 (-1) version;

      # Target platforms for Go binaries
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

      # OCI base image
      imageBase = {
        name = "gcr.io/distroless/static";
        digest = "sha256:b7b9a6953e7bed6baaf37329331051d7bdc1b99c885f6dbeb72d75b1baad54f9";
      };

      # Target architectures for OCI images (hash is the per-arch NAR hash of the pulled base image)
      imageArchs = [
        {
          arch = "amd64";
          hash = "sha256-3lqT9uvd6ibN2j/vtib9GrKJdmGToT9I+Vf5elQZD/4=";
        }
        {
          arch = "arm64";
          hash = "sha256-HcNRmnBYbd4yk8FLdKUn3ycqeTAQVfbOdRRPKdPverg=";
        }
        {
          arch = "arm";
          hash = "sha256-1gAbYf7CW5BISbReBh7rhexlsbvATL0R2ZfMoG8+duc=";
        }
        {
          arch = "ppc64le";
          hash = "sha256-1+NLWURPb6ayHw05BqtzSu7+XsZ/d8jjP4C2tB0jBak=";
        }
      ];

      # Build a Go binary for a specific GOOS/GOARCH using Go's native cross-compilation
      # This works from any host because CGO_ENABLED=0 means pure Go compilation
      mkGoBinary =
        {
          pkgs,
          pname,
          subPackage,
          platform,
        }:
        let
          ext = if platform.os == "windows" then ".exe" else "";
        in
        pkgs.buildGoApplication {
          pname = "${pname}-${platform.os}-${platform.arch}";
          inherit version;
          src = self;
          pwd = self;
          modules = ./gomod2nix.toml;
          subPackages = [ subPackage ];

          go = pkgs.go // {
            GOOS = platform.os;
            GOARCH = platform.arch;
          };

          # Disable CGO for cross-compilation
          CGO_ENABLED = "0";

          # Don't run tests during cross-compilation
          doCheck = false;

          # Set ldflags via shell variable instead of the ldflags attribute. The
          # ldflags attribute is passed to gomod2nix's mkGoCacheEnv, which would
          # cause the Go build cache derivation hash to change on every commit
          # (since ldflags includes the version, which includes the git commit).
          # The gomod2nix build hook reads the ldflags shell variable directly.
          preBuild = ''
            ldflags="-s -w -X=github.com/crossplane/crossplane/v2/internal/version.version=${version}"
          '';

          postInstall = ''
            # Go cross-compilation puts binaries in bin/GOOS_GOARCH/
            if [ -d $out/bin/${platform.os}_${platform.arch} ]; then
              mv $out/bin/${platform.os}_${platform.arch}/* $out/bin/
              rmdir $out/bin/${platform.os}_${platform.arch}
            fi
            cd $out/bin
            sha256sum ${pname}${ext} | head -c 64 > ${pname}${ext}.sha256
          '';

          meta = {
            description = "Crossplane - The cloud native control plane framework";
            homepage = "https://crossplane.io";
            license = pkgs.lib.licenses.asl20;
            mainProgram = pname;
          };
        };

      # Build crank tarball (tar.gz with checksums)
      mkCrankBundle =
        {
          pkgs,
          crankDrv,
          platform,
        }:
        let
          ext = if platform.os == "windows" then ".exe" else "";
        in
        pkgs.runCommand "crank-bundle-${platform.os}-${platform.arch}-${version}"
          {
            nativeBuildInputs = [
              pkgs.gnutar
              pkgs.gzip
            ];
          }
          ''
            mkdir -p $out
            cp ${crankDrv}/bin/crank${ext} .
            cp ${crankDrv}/bin/crank${ext}.sha256 .
            tar -czvf $out/crank.tar.gz crank${ext} crank${ext}.sha256
            cd $out
            sha256sum crank.tar.gz | head -c 64 > crank.tar.gz.sha256
          '';

      # Build OCI image for a specific architecture (always Linux)
      mkImage =
        {
          pkgs,
          crossplaneBin,
          arch,
          hash,
        }:
        let
          base = pkgs.dockerTools.pullImage {
            imageName = imageBase.name;
            imageDigest = imageBase.digest;
            inherit arch hash;
            os = "linux";
          };
          rawImage = pkgs.dockerTools.buildLayeredImage {
            name = "crossplane/crossplane";
            tag = version;
            created = "now";
            architecture = arch;

            fromImage = base;

            contents = [
              crossplaneBin
            ];

            extraCommands = ''
              mkdir -p crds webhookconfigurations
              cp -r ${self}/cluster/crds/* crds/
              cp -r ${self}/cluster/webhookconfigurations/* webhookconfigurations/
            '';

            config = {
              Entrypoint = [ "/bin/crossplane" ];
              ExposedPorts = {
                "8080/tcp" = { };
              };
              User = "65532";
              Labels = {
                "org.opencontainers.image.source" = "https://github.com/crossplane/crossplane";
                "org.opencontainers.image.version" = version;
              };
            };
          };
        in
        rawImage;

      mkHelmChart =
        pkgs:
        pkgs.runCommand "crossplane-helm-chart-${chartVersion}"
          {
            nativeBuildInputs = [ pkgs.kubernetes-helm ];
          }
          ''
            mkdir -p $out
            cp -r ${self}/cluster/charts/crossplane chart
            chmod -R u+w chart
            cd chart
            helm dependency update 2>/dev/null || true
            helm package --version ${chartVersion} --app-version ${chartVersion} -d $out .
          '';

    in
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system overlays;
        };

        crossplaneBins = builtins.listToAttrs (
          map (platform: {
            name = "${platform.os}-${platform.arch}";
            value = mkGoBinary {
              inherit pkgs platform;
              pname = "crossplane";
              subPackage = "cmd/crossplane";
            };
          }) goPlatforms
        );

        crankBins = builtins.listToAttrs (
          map (platform: {
            name = "${platform.os}-${platform.arch}";
            value = mkGoBinary {
              inherit pkgs platform;
              pname = "crank";
              subPackage = "cmd/crank";
            };
          }) goPlatforms
        );

        crankBundles = builtins.listToAttrs (
          map (platform: {
            name = "${platform.os}-${platform.arch}";
            value = mkCrankBundle {
              inherit pkgs platform;
              crankDrv = crankBins."${platform.os}-${platform.arch}";
            };
          }) goPlatforms
        );

        images = builtins.listToAttrs (
          map (platform: {
            name = "linux-${platform.arch}";
            value = mkImage {
              inherit pkgs;
              inherit (platform) arch hash;
              crossplaneBin = crossplaneBins."linux-${platform.arch}";
            };
          }) imageArchs
        );

        nativePlatform = {
          os = if pkgs.stdenv.isDarwin then "darwin" else "linux";
          arch = if pkgs.stdenv.hostPlatform.isAarch64 then "arm64" else "amd64";
        };

        # Code generation tools (used in generate app, check, and devShell)
        codegenTools = [
          pkgs.buf
          pkgs.goverter
          pkgs.protoc-gen-go
          pkgs.protoc-gen-go-grpc
          pkgs.kubernetes-controller-tools
        ];

        # E2E test binary - built with buildGoApplication for caching
        e2e = pkgs.buildGoApplication {
          pname = "crossplane-e2e";
          inherit version;
          src = self;
          pwd = self;
          modules = ./gomod2nix.toml;

          # Build test binary instead of regular binary
          buildPhase = ''
            runHook preBuild
            go test -c -o e2e ./test/e2e
            runHook postBuild
          '';

          installPhase = ''
            mkdir -p $out/bin
            cp e2e $out/bin/
          '';

          # Don't run the tests during the build
          doCheck = false;
        };

      in
      {
        # Packages are slow, but pure. They're mostly for use in CI. Nix builds
        # them in a sandbox without network or filesystem access. This makes
        # them fully reproducible. The same inputs will always produce the same
        # outputs. They're slower than apps mostly because they don't benefit
        # as much from the Go build cache.
        #
        # Run with: nix build
        packages = {
          default = pkgs.runCommand "crossplane-release-${version}" { } ''
            mkdir -p $out/bin $out/bundle $out/charts $out/images

            ${pkgs.lib.concatMapStrings (p: ''
              mkdir -p $out/bin/${p.os}_${p.arch}
              cp ${crossplaneBins."${p.os}-${p.arch}"}/bin/* $out/bin/${p.os}_${p.arch}/
              cp ${crankBins."${p.os}-${p.arch}"}/bin/* $out/bin/${p.os}_${p.arch}/
            '') goPlatforms}

            ${pkgs.lib.concatMapStrings (p: ''
              mkdir -p $out/bundle/${p.os}_${p.arch}
              cp ${crankBundles."${p.os}-${p.arch}"}/* $out/bundle/${p.os}_${p.arch}/
            '') goPlatforms}

            cp ${mkHelmChart pkgs}/* $out/charts/

            ${pkgs.lib.concatMapStrings (p: ''
              mkdir -p $out/images/linux_${p.arch}
              cp ${images."linux-${p.arch}"} $out/images/linux_${p.arch}/image.tar.gz
            '') imageArchs}
          '';
        };

        # Checks are slow, but pure. They're mostly for use in CI. Nix runs them
        # in a sandbox without network or filesystem access. This makes them
        # fully reproducible. If a check passes for a particular commit at one
        # point it time, it should always pass for that commit. They're slower
        # than apps mostly because they don't benefit as much from the build
        # cache.
        #
        # Run all with: nix flake check
        # Run one with: nix build .#checks.$(nix config show system).test
        checks = {
          # Run Go unit tests with coverage
          test = pkgs.buildGoApplication {
            pname = "crossplane-test";
            inherit version;
            src = self;
            pwd = self;
            modules = ./gomod2nix.toml;

            dontBuild = true;

            # Excludes e2e tests
            checkPhase = ''
              runHook preCheck
              export HOME=$TMPDIR
              go test -covermode=count -coverprofile=coverage.txt ./apis/... ./cmd/... ./internal/...
              runHook postCheck
            '';

            installPhase = ''
              mkdir -p $out
              cp coverage.txt $out/
            '';
          };

          # Run Go linter (without --fix)
          go-lint = pkgs.buildGoApplication {
            pname = "crossplane-go-lint";
            inherit version;
            src = self;
            pwd = self;
            modules = ./gomod2nix.toml;

            nativeBuildInputs = [ pkgs.golangci-lint ];

            dontBuild = true;

            checkPhase = ''
              runHook preCheck
              export HOME=$TMPDIR
              export GOLANGCI_LINT_CACHE=$TMPDIR/.cache/golangci-lint
              golangci-lint run
              runHook postCheck
            '';

            installPhase = ''
              mkdir -p $out
              touch $out/.lint-passed
            '';
          };

          # Run Helm linter
          helm-lint =
            pkgs.runCommand "crossplane-helm-lint"
              {
                nativeBuildInputs = [ pkgs.kubernetes-helm ];
              }
              ''
                cd ${self}/cluster/charts/crossplane
                helm lint .
                mkdir -p $out
                touch $out/.lint-passed
              '';

          # Verify generated code is up-to-date
          generate = pkgs.buildGoApplication {
            pname = "crossplane-generate-check";
            inherit version;
            src = self;
            pwd = self;
            modules = ./gomod2nix.toml;

            nativeBuildInputs = codegenTools ++ [ pkgs.kubectl ];

            dontBuild = true;

            checkPhase = ''
              runHook preCheck
              export HOME=$TMPDIR

              # Run code generation
              go generate -tags generate .

              # Patch CRDs
              kubectl patch --local --type=json \
                --patch-file cluster/crd-patches/pkg.crossplane.io_deploymentruntimeconfigs.yaml \
                --filename cluster/crds/pkg.crossplane.io_deploymentruntimeconfigs.yaml \
                --output=yaml > /tmp/patched.yaml \
                && mv /tmp/patched.yaml cluster/crds/pkg.crossplane.io_deploymentruntimeconfigs.yaml

              # Compare against committed source
              if ! diff -rq apis ${self}/apis > /dev/null 2>&1 || \
                 ! diff -rq internal ${self}/internal > /dev/null 2>&1 || \
                 ! diff -rq proto ${self}/proto > /dev/null 2>&1 || \
                 ! diff -rq cluster/crds ${self}/cluster/crds > /dev/null 2>&1 || \
                 ! diff -rq cluster/webhookconfigurations ${self}/cluster/webhookconfigurations > /dev/null 2>&1; then
                echo "ERROR: Generated code is out of date. Run 'nix run .#generate' and commit the changes."
                echo ""
                echo "Changed files:"
                diff -rq apis ${self}/apis 2>/dev/null || true
                diff -rq internal ${self}/internal 2>/dev/null || true
                diff -rq proto ${self}/proto 2>/dev/null || true
                diff -rq cluster/crds ${self}/cluster/crds 2>/dev/null || true
                diff -rq cluster/webhookconfigurations ${self}/cluster/webhookconfigurations 2>/dev/null || true
                exit 1
              fi

              runHook postCheck
            '';

            installPhase = ''
              mkdir -p $out
              touch $out/.generate-check-passed
            '';
          };
        };

        # Apps are fast, but impure. Unlike packages and checks, Nix runs apps in your
        # local environment rather than an isolated sandbox. They can access the
        # network, modify local files, and benefit from caches like GOCACHE. This makes
        # them non-reproducible, but much faster for day-to-day development.
        #
        # Run with: nix run .#test
        apps = {
          load-image = {
            type = "app";
            program = pkgs.lib.getExe (
              pkgs.writeShellScriptBin "load-image" ''
                set -e
                echo "Loading crossplane/crossplane:${version} (linux/${nativePlatform.arch}) into Docker..."
                ${pkgs.docker-client}/bin/docker load < ${images."linux-${nativePlatform.arch}"}
              ''
            );
            meta.description = "Build and load OCI image into Docker";
          };

          test = {
            type = "app";
            program = pkgs.lib.getExe (
              pkgs.writeShellScriptBin "test" ''
                set -e
                ${pkgs.go}/bin/go test -covermode=count ./apis/... ./cmd/... ./internal/... "$@"
              ''
            );
            meta.description = "Run unit tests";
          };

          lint = {
            type = "app";
            program = pkgs.lib.getExe (
              pkgs.writeShellScriptBin "lint" ''
                set -e
                export GOLANGCI_LINT_CACHE="''${XDG_CACHE_HOME:-$HOME/.cache}/golangci-lint"
                ${pkgs.golangci-lint}/bin/golangci-lint run --fix "$@"
              ''
            );
            meta.description = "Run golangci-lint with auto-fix";
          };

          tidy = {
            type = "app";
            program = pkgs.lib.getExe (
              pkgs.writeShellScriptBin "tidy" ''
                set -e
                echo "Running go mod tidy..."
                ${pkgs.go}/bin/go mod tidy
                echo "Regenerating gomod2nix.toml..."
                ${gomod2nix.packages.${system}.default}/bin/gomod2nix generate --with-deps
                echo "Done"
              ''
            );
            meta.description = "Run go mod tidy and regenerate gomod2nix.toml";
          };

          generate = {
            type = "app";
            program = pkgs.lib.getExe (
              pkgs.writeShellScriptBin "generate" ''
                set -e
                # PATH injection required - go generate spawns tools that must be discoverable
                export PATH="${
                  pkgs.lib.makeBinPath (
                    [
                      pkgs.coreutils
                      pkgs.go
                      pkgs.kubectl
                    ]
                    ++ codegenTools
                  )
                }:$PATH"

                echo "Running go generate..."
                ${pkgs.go}/bin/go generate -tags generate .

                echo "Patching CRDs..."
                ${pkgs.kubectl}/bin/kubectl patch --local --type=json \
                  --patch-file cluster/crd-patches/pkg.crossplane.io_deploymentruntimeconfigs.yaml \
                  --filename cluster/crds/pkg.crossplane.io_deploymentruntimeconfigs.yaml \
                  --output=yaml > /tmp/patched.yaml \
                  && mv /tmp/patched.yaml cluster/crds/pkg.crossplane.io_deploymentruntimeconfigs.yaml

                echo "Done"
              ''
            );
            meta.description = "Run code generation";
          };

          e2e = {
            type = "app";
            program = pkgs.lib.getExe (
              pkgs.writeShellScriptBin "e2e" ''
                set -e

                echo "Loading crossplane image into Docker..."
                ${pkgs.docker-client}/bin/docker load < ${images."linux-${nativePlatform.arch}"}

                echo "Tagging image as crossplane-e2e/crossplane:latest..."
                ${pkgs.docker-client}/bin/docker tag crossplane/crossplane:${version} crossplane-e2e/crossplane:latest

                echo "Running e2e tests..."
                ${pkgs.gotestsum}/bin/gotestsum \
                  --format standard-verbose \
                  --raw-command -- ${pkgs.go}/bin/go tool test2json -t -p E2E ${e2e}/bin/e2e -test.v "$@"
              ''
            );
            meta.description = "Run end-to-end tests";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.go
            pkgs.golangci-lint
            pkgs.kubectl
            pkgs.kubernetes-helm
            pkgs.kind
            pkgs.gotestsum
            pkgs.helm-docs
            gomod2nix.packages.${system}.default
          ]
          ++ codegenTools;

          shellHook = ''
            echo "Crossplane development shell ($(go version | cut -d' ' -f3))"
            echo ""
            echo "Local development:"
            echo "  nix run .#load-image                # Build and load OCI image into Docker"
            echo "  nix run .#generate                  # Run code generation"
            echo "  nix run .#lint                      # Run linter (auto-fixes)"
            echo "  nix run .#test                      # Run unit tests"
            echo "  nix run .#tidy                      # Tidy Go modules"
            echo "  nix run .#e2e -- -test.run TestFoo  # Run E2E tests"
            echo ""
            echo "CI:"
            echo "  nix build                           # All binaries, images, Helm chart"
            echo "  nix flake check                     # Run all checks (test, lint, generate)"
            echo ""
            echo "To use your preferred shell: nix develop -c \$SHELL"
            echo ""
          '';
        };

      }
    );
}
