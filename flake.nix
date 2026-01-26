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

      # Set automatically by CI (see .github/workflows/ci.yml).
      #
      # To perfectly reproduce a CI build locally:
      # 1. Checkout the relevant commit
      # 2. Set buildVersion to the relevant release version
      # 3. Run ./nix.sh build
      buildVersion = "";

      # Auto-generated version for local development builds.
      devVersion =
        if self ? shortRev then
          "v0.0.0-${builtins.toString self.lastModified}-${self.shortRev}"
        else
          "v0.0.0-${builtins.toString self.lastModified}-${self.dirtyShortRev}";

      version = if buildVersion != "" then buildVersion else devVersion;

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

      # Target architectures for OCI images
      imageArchs = [
        "amd64"
        "arm64"
        "arm"
        "ppc64le"
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
      # This replicates gcr.io/distroless/static using pure Nix.
      mkImageArgs =
        {
          pkgs,
          crossplaneBin,
          arch,
        }:
        let
          passwd = pkgs.writeText "passwd" ''
            root:x:0:0:root:/root:/sbin/nologin
            nobody:x:65534:65534:nobody:/nonexistent:/sbin/nologin
            nonroot:x:65532:65532:nonroot:/home/nonroot:/sbin/nologin
          '';
          group = pkgs.writeText "group" ''
            root:x:0:
            nobody:x:65534:
            nonroot:x:65532:
          '';
          nsswitch = pkgs.writeText "nsswitch.conf" ''
            hosts: files dns
          '';
        in
        {
          name = "crossplane/crossplane";
          tag = version;
          created = "now";
          architecture = arch;

          contents = [
            crossplaneBin
            pkgs.cacert
            pkgs.tzdata
            pkgs.iana-etc
          ];

          extraCommands = ''
            mkdir -p tmp home/nonroot etc crds webhookconfigurations
            chmod 1777 tmp
            cp ${passwd} etc/passwd
            cp ${group} etc/group
            cp ${nsswitch} etc/nsswitch.conf
            cp -r ${self}/cluster/crds/* crds/
            cp -r ${self}/cluster/webhookconfigurations/* webhookconfigurations/
          '';

          config = {
            Entrypoint = [ "/bin/crossplane" ];
            ExposedPorts = {
              "8080/tcp" = { };
            };
            User = "65532";
            Env = [
              "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
              "SSL_CERT_FILE=${pkgs.cacert}/etc/ssl/certs/ca-certificates.crt"
            ];
            Labels = {
              "org.opencontainers.image.source" = "https://github.com/crossplane/crossplane";
              "org.opencontainers.image.version" = version;
            };
          };
        };

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
          map (arch: {
            name = "linux-${arch}";
            value = pkgs.dockerTools.buildLayeredImage (mkImageArgs {
              inherit pkgs arch;
              crossplaneBin = crossplaneBins."linux-${arch}";
            });
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

        # Development tools shared by apps and devShell
        devTools = [
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
          pkgs.helm-docs
          gomod2nix.packages.${system}.default
        ]
        ++ codegenTools;

        # PATH for apps - isolated from host tools
        devToolsPath = pkgs.lib.makeBinPath devTools;

        # E2E test binary - built with buildGoApplication for caching
        e2e = pkgs.buildGoApplication {
          pname = "crossplane-e2e";
          inherit version;
          src = self;
          pwd = self;
          modules = ./gomod2nix.toml;

          CGO_ENABLED = "0";

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

            ${pkgs.lib.concatMapStrings (arch: ''
              mkdir -p $out/images/linux_${arch}
              cp ${images."linux-${arch}"} $out/images/linux_${arch}/image.tar.gz
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

            CGO_ENABLED = "0";

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

            CGO_ENABLED = "0";

            nativeBuildInputs = devTools;

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
                nativeBuildInputs = devTools;
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

            CGO_ENABLED = "0";

            nativeBuildInputs = devTools;

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

              # Generate Helm chart docs
              helm-docs --chart-search-root=cluster/charts

              # Compare against committed source
              if ! diff -rq apis ${self}/apis > /dev/null 2>&1 || \
                 ! diff -rq internal ${self}/internal > /dev/null 2>&1 || \
                 ! diff -rq proto ${self}/proto > /dev/null 2>&1 || \
                 ! diff -rq cluster/crds ${self}/cluster/crds > /dev/null 2>&1 || \
                 ! diff -rq cluster/webhookconfigurations ${self}/cluster/webhookconfigurations > /dev/null 2>&1 || \
                 ! diff -rq cluster/charts ${self}/cluster/charts > /dev/null 2>&1; then
                echo "ERROR: Generated code is out of date. Run 'nix run .#generate' and commit the changes."
                echo ""
                echo "Changed files:"
                diff -rq apis ${self}/apis 2>/dev/null || true
                diff -rq internal ${self}/internal 2>/dev/null || true
                diff -rq proto ${self}/proto 2>/dev/null || true
                diff -rq cluster/crds ${self}/cluster/crds 2>/dev/null || true
                diff -rq cluster/webhookconfigurations ${self}/cluster/webhookconfigurations 2>/dev/null || true
                diff -rq cluster/charts ${self}/cluster/charts 2>/dev/null || true
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
          stream-image =
            let
              streamScript = pkgs.dockerTools.streamLayeredImage (mkImageArgs {
                inherit pkgs;
                arch = nativePlatform.arch;
                crossplaneBin = crossplaneBins."linux-${nativePlatform.arch}";
              });
            in
            {
              type = "app";
              program = "${streamScript}";
              meta.description = "Stream OCI image tarball to stdout (pipe to docker load)";
            };

          test = {
            type = "app";
            program = pkgs.lib.getExe (
              pkgs.writeShellApplication {
                name = "test";
                text = ''
                  export PATH="${devToolsPath}"
                  export CGO_ENABLED=0
                  go test -covermode=count ./apis/... ./cmd/... ./internal/... "$@"
                '';
              }
            );
            meta.description = "Run unit tests";
          };

          lint = {
            type = "app";
            program = pkgs.lib.getExe (
              pkgs.writeShellApplication {
                name = "lint";
                text = ''
                  export PATH="${devToolsPath}"
                  export CGO_ENABLED=0
                  export GOLANGCI_LINT_CACHE="''${XDG_CACHE_HOME:-$HOME/.cache}/golangci-lint"
                  golangci-lint run --fix "$@"
                '';
              }
            );
            meta.description = "Run golangci-lint with auto-fix";
          };

          tidy = {
            type = "app";
            program = pkgs.lib.getExe (
              pkgs.writeShellApplication {
                name = "tidy";
                text = ''
                  export PATH="${devToolsPath}"
                  export CGO_ENABLED=0
                  echo "Running go mod tidy..."
                  go mod tidy
                  echo "Regenerating gomod2nix.toml..."
                  gomod2nix generate --with-deps
                  echo "Done"
                '';
              }
            );
            meta.description = "Run go mod tidy and regenerate gomod2nix.toml";
          };

          generate = {
            type = "app";
            program = pkgs.lib.getExe (
              pkgs.writeShellApplication {
                name = "generate";
                text = ''
                  export PATH="${devToolsPath}"
                  export CGO_ENABLED=0

                  echo "Running go generate..."
                  go generate -tags generate .

                  echo "Patching CRDs..."
                  kubectl patch --local --type=json \
                    --patch-file cluster/crd-patches/pkg.crossplane.io_deploymentruntimeconfigs.yaml \
                    --filename cluster/crds/pkg.crossplane.io_deploymentruntimeconfigs.yaml \
                    --output=yaml > /tmp/patched.yaml \
                    && mv /tmp/patched.yaml cluster/crds/pkg.crossplane.io_deploymentruntimeconfigs.yaml

                  echo "Generating Helm chart docs..."
                  helm-docs --chart-search-root=cluster/charts

                  echo "Done"
                '';
              }
            );
            meta.description = "Run code generation";
          };

          e2e = {
            type = "app";
            program = pkgs.lib.getExe (
              pkgs.writeShellApplication {
                name = "e2e";
                text = ''
                  export PATH="${devToolsPath}"
                  export CGO_ENABLED=0

                  JUNIT_DIR="''${TMPDIR:-/tmp}"

                  echo "Loading crossplane image into Docker..."
                  docker load < ${images."linux-${nativePlatform.arch}"}

                  echo "Tagging image as crossplane-e2e/crossplane:latest..."
                  docker tag crossplane/crossplane:${version} crossplane-e2e/crossplane:latest

                  echo "Running e2e tests..."
                  gotestsum \
                    --format standard-verbose \
                    --junitfile "$JUNIT_DIR/e2e-tests.xml" \
                    --raw-command -- go tool test2json -t -p E2E ${e2e}/bin/e2e -test.v "$@"
                '';
              }
            );
            meta.description = "Run end-to-end tests";
          };

          hack = {
            type = "app";
            program = pkgs.lib.getExe (
              pkgs.writeShellApplication {
                name = "hack";
                runtimeInputs = devTools;
                text = ''
                  CLUSTER_NAME="crossplane-hack"

                  # (Re)create cluster if control plane isn't running
                  if ! docker ps --format '{{.Names}}' | grep -q "^$CLUSTER_NAME-control-plane$"; then
                    kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
                    echo "Creating kind cluster..."
                    kind create cluster --name "$CLUSTER_NAME" --wait 60s
                  fi

                  echo "Loading Crossplane image..."
                  docker load < ${images."linux-${nativePlatform.arch}"}
                  kind load docker-image --name "$CLUSTER_NAME" crossplane/crossplane:${version}

                  echo "Installing Crossplane..."
                  helm upgrade --install crossplane ${mkHelmChart pkgs}/crossplane-${chartVersion}.tgz \
                    --namespace crossplane-system --create-namespace \
                    --set image.pullPolicy=Never \
                    --set image.repository=crossplane/crossplane \
                    --set image.tag=${version} \
                    --set "args={--debug}" \
                    --wait

                  echo ""
                  echo "Crossplane is running in kind cluster '$CLUSTER_NAME'."
                  kubectl get pods -n crossplane-system
                '';
              }
            );
            meta.description = "Create kind cluster with Crossplane for local development";
          };

          push-images = {
            type = "app";
            program = pkgs.lib.getExe (
              pkgs.writeShellApplication {
                name = "push-images";
                runtimeInputs = [ pkgs.docker-client ];
                text = ''
                  REPO="''${1:?Usage: nix run .#push-images -- <registry/image>}"

                  echo "Pushing images to ''${REPO}..."

                  # Load, tag, and push each architecture
                  ${pkgs.lib.concatMapStrings (arch: ''
                    echo "Loading and pushing ''${REPO}:${version}-${arch}..."
                    docker load < ${images."linux-${arch}"}
                    docker tag crossplane/crossplane:${version} "''${REPO}:${version}-${arch}"
                    docker push "''${REPO}:${version}-${arch}"
                  '') imageArchs}

                  # Create and push multi-arch manifest
                  echo "Creating manifest ''${REPO}:${version}..."
                  docker manifest create "''${REPO}:${version}" \
                    ${pkgs.lib.concatMapStringsSep " " (arch: ''"''${REPO}:${version}-${arch}"'') imageArchs}
                  docker manifest push "''${REPO}:${version}"

                  echo "Pushed ''${REPO}:${version}"
                '';
              }
            );
            meta.description = "Push multi-arch images to a container registry";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = devTools;

          shellHook = ''
            export PS1='\[\033[38;2;243;128;123m\][cros\[\033[38;2;255;205;60m\]spla\[\033[38;2;53;208;186m\]ne]\[\033[0m\] \w \$ '

            export HISTFILE="/nix/.bash_history"
            export HISTSIZE=10000
            export HISTFILESIZE=10000

            source <(kubectl completion bash 2>/dev/null)
            source <(helm completion bash 2>/dev/null)
            source <(kind completion bash 2>/dev/null)

            alias k=kubectl
            complete -o default -F __start_kubectl k

            echo "Crossplane development shell ($(go version | cut -d' ' -f3))"
            echo ""
            echo "Local development:"
            echo "  nix run .#stream-image | docker load   # Load OCI image into Docker"
            echo "  nix run .#generate                     # Run code generation"
            echo "  nix run .#lint                         # Run linter (auto-fixes)"
            echo "  nix run .#test                         # Run unit tests"
            echo "  nix run .#tidy                         # Tidy Go modules"
            echo "  nix run .#e2e -- -test.run TestFoo     # Run E2E tests"
            echo "  nix run .#hack                         # Deploy Crossplane to a kind cluster"
            echo ""
            echo "CI:"
            echo "  nix build                              # All binaries, images, Helm chart"
            echo "  nix flake check                        # Run all checks (test, lint, generate)"
            echo ""
            echo "Install extra tools (https://search.nixos.org/packages):"
            echo "  nix-env -iA nixpkgs.neovim nixpkgs.jq"
            echo ""
          '';
        };

      }
    );
}
