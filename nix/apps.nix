# Interactive development commands for Crossplane.
#
# Apps run outside the Nix sandbox with full filesystem and network access.
# They're designed for local development where Go modules are already available.
#
# All apps are builder functions that take an attrset of arguments and return a
# complete app definition ({ type, meta.description, program }). Most use
# writeShellApplication to create the program. The text block is preprocessed:
#
#   ${somePkg}/bin/foo   -> /nix/store/.../bin/foo  (Nix store path)
#   ''${SOME_VAR}        -> ${SOME_VAR}             (shell variable, escaped)
#
# Each app declares its tool dependencies via runtimeInputs, with inheritPath
# set to false. This ensures apps only use explicitly declared tools.
{ pkgs }:
{
  # Run Go unit tests.
  test = _: {
    type = "app";
    meta.description = "Run unit tests";
    program = pkgs.lib.getExe (
      pkgs.writeShellApplication {
        name = "crossplane-test";
        runtimeInputs = [ pkgs.go ];
        inheritPath = false;
        text = ''
          export CGO_ENABLED=0
          go test -covermode=count github.com/crossplane/crossplane/apis/v2/... "$@"
          go test -covermode=count ./cmd/... ./internal/... "$@"
        '';
      }
    );
  };

  # Run linters with auto-fix. Formats code first, then reports remaining issues.
  lint = _: {
    type = "app";
    meta.description = "Format code and run linters";
    program = pkgs.lib.getExe (
      pkgs.writeShellApplication {
        name = "crossplane-lint";
        runtimeInputs = [
          pkgs.findutils
          pkgs.go
          pkgs.golangci-lint
          pkgs.statix
          pkgs.deadnix
          pkgs.nixfmt-rfc-style
          pkgs.shellcheck
          pkgs.gnupatch
          pkgs.shfmt
        ];
        inheritPath = false;
        text = ''
          export CGO_ENABLED=0
          export GOLANGCI_LINT_CACHE="''${XDG_CACHE_HOME:-$HOME/.cache}/golangci-lint"

          echo "Formatting and linting Nix..."
          statix fix .
          deadnix --edit flake.nix nix/*.nix
          nixfmt flake.nix nix/*.nix

          echo "Formatting and linting shell..."
          find . -name '*.sh' -type f | while read -r script; do
            shellcheck --format=diff "$script" | patch -p1 || true
            shfmt -w "$script"
          done
          find . -name '*.sh' -type f -exec shellcheck {} +

          echo "Formatting and linting Go..."
          golangci-lint run --fix "$@"
          cd apis && golangci-lint run --config=../.golangci.yml --fix "$@"
        '';
      }
    );
  };

  # Run code generation.
  generate = _: {
    type = "app";
    meta.description = "Run code generation";
    program = pkgs.lib.getExe (
      pkgs.writeShellApplication {
        name = "crossplane-generate";
        runtimeInputs = [
          pkgs.coreutils
          pkgs.gnused
          pkgs.go
          pkgs.kubectl
          pkgs.helm-docs

          # Code generation
          pkgs.buf
          pkgs.goverter
          pkgs.protoc-gen-go
          pkgs.protoc-gen-go-grpc
          pkgs.kubernetes-controller-tools
        ];
        inheritPath = false;
        text = ''
          export CGO_ENABLED=0

          echo "Running go generate..."
          go generate -tags generate .

          echo "Running go generate in apis/..."
          pushd apis && go generate -tags generate . && popd

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
  };

  # Run go mod tidy and regenerate gomod2nix.toml.
  tidy = _: {
    type = "app";
    meta.description = "Run go mod tidy and regenerate gomod2nix.toml";
    program = pkgs.lib.getExe (
      pkgs.writeShellApplication {
        name = "crossplane-tidy";
        runtimeInputs = [
          pkgs.go
          pkgs.gomod2nix
        ];
        inheritPath = false;
        text = ''
          export CGO_ENABLED=0

          echo "Running go mod tidy..."
          go mod tidy
          echo "Regenerating gomod2nix.toml..."
          gomod2nix generate --with-deps

          echo "Running go mod tidy for apis/..."
          cd apis
          go mod tidy
          echo "Regenerating apis/gomod2nix.toml..."
          gomod2nix generate --with-deps

          echo "Done"
        '';
      }
    );
  };

  # Stream OCI image tarball to stdout (pipe to docker load).
  streamImage =
    { imageArgs }:
    {
      type = "app";
      meta.description = "Stream OCI image tarball to stdout (pipe to docker load)";
      program = "${pkgs.dockerTools.streamLayeredImage imageArgs}";
    };

  # Run end-to-end tests.
  e2e =
    {
      image,
      bin,
      version,
    }:
    {
      type = "app";
      meta.description = "Run end-to-end tests";
      program = pkgs.lib.getExe (
        pkgs.writeShellApplication {
          name = "crossplane-e2e";
          runtimeInputs = [
            pkgs.coreutils
            pkgs.go
            pkgs.docker-client
            pkgs.gotestsum
            pkgs.kind
            pkgs.kubernetes-helm
          ];
          inheritPath = false;
          text = ''
            export CGO_ENABLED=0

            echo "Loading crossplane image into Docker..."
            docker load < ${image}

            echo "Tagging image as crossplane-e2e/crossplane:latest..."
            docker tag crossplane/crossplane:${version} crossplane-e2e/crossplane:latest

            echo "Running e2e tests..."
            gotestsum \
              --format standard-verbose \
              --junitfile "''${TMPDIR:-/tmp}/e2e-tests.xml" \
              --raw-command -- go tool test2json -t -p E2E ${bin}/bin/e2e -test.v "$@"
          '';
        }
      );
    };

  # Create kind cluster with Crossplane for local development.
  hack =
    {
      image,
      chart,
      version,
    }:
    let
      chartVersion = builtins.substring 1 (-1) version;
    in
    {
      type = "app";
      meta.description = "Create kind cluster with Crossplane for local development";
      program = pkgs.lib.getExe (
        pkgs.writeShellApplication {
          name = "crossplane-hack";
          runtimeInputs = [
            pkgs.coreutils
            pkgs.docopts
            pkgs.gnugrep
            pkgs.docker-client
            pkgs.kind
            pkgs.kubectl
            pkgs.kubernetes-helm
            pkgs.nix
          ];
          inheritPath = false;
          text = ''
            DOC='Usage: hack [options]

            Create a kind cluster with Crossplane for local development.

            Options:
              -n, --cluster NAME  Kind cluster name [default: crossplane-hack]
              -a, --args ARGS     Comma-separated arguments for the Crossplane pod [default: --debug]
              -f, --values FILE   Path to a Helm values file for Crossplane configuration
              -h, --help          Show this help message

            Examples:
              nix run .#hack
              nix run .#hack -- --args="--debug,--enable-operations"
              nix run .#hack -- --values ./my-values.yaml
              nix run .#hack -- --cluster my-cluster
            '

            # docopts parses args per the DOC usage string, setting $cluster, $args, etc.
            eval "$(docopts -h "$DOC" : "$@")"

            # shellcheck disable=SC2154 # $cluster set by docopts eval.
            if ! docker ps --format '{{.Names}}' | grep -q "^$cluster-control-plane$"; then
              kind delete cluster --name "$cluster" 2>/dev/null || true
              echo "Creating kind cluster..."
              kind create cluster --name "$cluster" --wait 60s
            fi

            # Ensure kubeconfig is set up for existing clusters.
            kind export kubeconfig --name "$cluster"

            echo "Loading Crossplane image..."
            docker load < ${image}
            kind load docker-image --name "$cluster" crossplane/crossplane:${version}

            echo "Installing Crossplane..."
            # shellcheck disable=SC2154 # $args and $values set by docopts eval.
            helm upgrade --install crossplane ${chart}/crossplane-${chartVersion}.tgz \
              --namespace crossplane-system --create-namespace \
              --set image.pullPolicy=Never \
              --set image.repository=crossplane/crossplane \
              --set image.tag=${version} \
              --set "args={$args}" \
              ''${values:+-f "$values"} \
              --wait

            echo ""
            echo "Crossplane is running in kind cluster '$cluster'."
            kubectl get pods -n crossplane-system

            # When running via nix.sh, the cluster is inside the container. Drop
            # into a dev shell so the user can interact with it before exiting.
            if [ "''${NIX_SH_CONTAINER:-}" = "1" ]; then
              echo ""
              echo "Entering development shell (exit to stop)..."
              exec nix develop
            fi
          '';
        }
      );
    };

  # Delete the kind cluster created by hack.
  unhack = _: {
    type = "app";
    meta.description = "Delete the kind cluster created by hack";
    program = pkgs.lib.getExe (
      pkgs.writeShellApplication {
        name = "crossplane-unhack";
        runtimeInputs = [
          pkgs.docopts
          pkgs.docker-client
          pkgs.kind
        ];
        inheritPath = false;
        text = ''
          DOC='Usage: unhack [options]

          Delete the kind cluster created by hack.

          Options:
            -n, --cluster NAME  Kind cluster name [default: crossplane-hack]
            -h, --help          Show this help message
          '

          # docopts parses args per the DOC usage string, setting $cluster.
          eval "$(docopts -h "$DOC" : "$@")"

          # shellcheck disable=SC2154 # $cluster set by docopts eval.
          kind delete clusters "$cluster"
        '';
      }
    );
  };

  # Push multi-arch images to a container registry.
  pushImages =
    {
      images,
      platforms,
      version,
    }:
    {
      type = "app";
      meta.description = "Push multi-arch images to a container registry";
      program = pkgs.lib.getExe (
        pkgs.writeShellApplication {
          name = "crossplane-push-images";
          runtimeInputs = [ pkgs.docker-client ];
          inheritPath = false;
          text = ''
            REPO="''${1:?Usage: nix run .#push-images -- <registry/image>}"

            echo "Pushing images to ''${REPO}..."
            ${pkgs.lib.concatMapStrings (p: ''
              echo "Loading and pushing ''${REPO}:${version}-${p.arch}..."
              docker load < ${images."${p.os}-${p.arch}".image}
              docker tag crossplane/crossplane:${version} "''${REPO}:${version}-${p.arch}"
              docker push "''${REPO}:${version}-${p.arch}"
            '') platforms}

            echo "Creating manifest ''${REPO}:${version}..."
            docker manifest create "''${REPO}:${version}" \
              ${pkgs.lib.concatMapStringsSep " " (p: ''"''${REPO}:${version}-${p.arch}"'') platforms}
            docker manifest push "''${REPO}:${version}"

            echo "Pushed ''${REPO}:${version}"
          '';
        }
      );
    };

  # Push build artifacts to S3.
  pushArtifacts =
    { release, version }:
    {
      type = "app";
      meta.description = "Push build artifacts to S3";
      program = pkgs.lib.getExe (
        pkgs.writeShellApplication {
          name = "crossplane-push-artifacts";
          runtimeInputs = [ pkgs.awscli2 ];
          inheritPath = false;
          text = ''
            BRANCH="''${1:?Usage: nix run .#push-artifacts -- <branch>}"

            echo "Pushing artifacts to s3://crossplane-releases/build/''${BRANCH}/${version}..."
            aws s3 sync --delete --only-show-errors \
              ${release} \
              "s3://crossplane-releases/build/''${BRANCH}/${version}"
            echo "Done"
          '';
        }
      );
    };

  # Promote images to a release channel.
  promoteImages = _: {
    type = "app";
    meta.description = "Promote images to a release channel";
    program = pkgs.lib.getExe (
      pkgs.writeShellApplication {
        name = "crossplane-promote-images";
        runtimeInputs = [ pkgs.docker-client ];
        inheritPath = false;
        text = ''
          REPO="''${1:?Usage: nix run .#promote-images -- <registry/image> <version> <channel>}"
          VERSION="''${2:?Usage: nix run .#promote-images -- <registry/image> <version> <channel>}"
          CHANNEL="''${3:?Usage: nix run .#promote-images -- <registry/image> <version> <channel>}"

          echo "Promoting ''${REPO}:''${VERSION} to channel ''${CHANNEL}..."
          docker buildx imagetools create \
            --tag "''${REPO}:''${CHANNEL}" \
            --tag "''${REPO}:''${VERSION}-''${CHANNEL}" \
            "''${REPO}:''${VERSION}"
          echo "Done"
        '';
      }
    );
  };

  # Promote build artifacts to a release channel.
  promoteArtifacts = _: {
    type = "app";
    meta.description = "Promote build artifacts to a release channel";
    program = pkgs.lib.getExe (
      pkgs.writeShellApplication {
        name = "crossplane-promote-artifacts";
        runtimeInputs = [
          pkgs.coreutils
          pkgs.awscli2
          pkgs.kubernetes-helm
        ];
        inheritPath = false;
        text = ''
          BRANCH="''${1:?Usage: nix run .#promote-artifacts -- <branch> <version> <channel> [--prerelease]}"
          VERSION="''${2:?Usage: nix run .#promote-artifacts -- <branch> <version> <channel> [--prerelease]}"
          CHANNEL="''${3:?Usage: nix run .#promote-artifacts -- <branch> <version> <channel> [--prerelease]}"
          PRERELEASE="''${4:-}"

          BUILD_PATH="s3://crossplane-releases/build/''${BRANCH}/''${VERSION}"
          CHANNEL_PATH="s3://crossplane-releases/''${CHANNEL}"
          CHARTS_PATH="s3://crossplane-helm-charts/''${CHANNEL}"

          WORKDIR=$(mktemp -d)
          trap 'rm -rf "$WORKDIR"' EXIT

          echo "Promoting artifacts from ''${BUILD_PATH} to ''${CHANNEL}..."

          aws s3 sync --only-show-errors "''${CHARTS_PATH}" "$WORKDIR/" || true
          aws s3 sync --only-show-errors "''${BUILD_PATH}/charts" "$WORKDIR/"
          helm repo index --url "https://charts.crossplane.io/''${CHANNEL}" "$WORKDIR/"
          aws s3 sync --delete --only-show-errors "$WORKDIR/" "''${CHARTS_PATH}"
          aws s3 cp --only-show-errors --cache-control "private, max-age=0, no-transform" \
            "$WORKDIR/index.yaml" "''${CHARTS_PATH}/index.yaml"

          aws s3 sync --delete --only-show-errors "''${BUILD_PATH}" "''${CHANNEL_PATH}/''${VERSION}"

          if [ "''${PRERELEASE}" != "--prerelease" ]; then
            aws s3 sync --delete --only-show-errors "''${BUILD_PATH}" "''${CHANNEL_PATH}/current"
          fi

          echo "Done"
        '';
      }
    );
  };
}
