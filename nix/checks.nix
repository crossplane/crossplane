# CI check builders for Crossplane.
#
# Checks run inside the Nix sandbox without network or filesystem access. This
# makes them fully reproducible but means Go modules must come from gomod2nix.
#
# Most checks use buildGoApplication, which sets up the Go environment with
# modules from gomod2nix.toml. This is different from apps, which run outside
# the sandbox and can access Go modules normally.
#
# All checks are builder functions that take an attrset of arguments and return
# a derivation. The actual check definitions live in flake.nix.
{ pkgs, self }:
{
  # Run Go unit tests with coverage
  test =
    { version }:
    pkgs.buildGoApplication {
      pname = "crossplane-test";
      inherit version;
      src = self;
      pwd = self;
      modules = ../gomod2nix.toml;

      CGO_ENABLED = "0";

      dontBuild = true;

      checkPhase = ''
        runHook preCheck
        export HOME=$TMPDIR
        go test -covermode=count -coverprofile=coverage.txt ./cmd/... ./internal/...
        runHook postCheck
      '';

      installPhase = ''
        mkdir -p $out
        cp coverage.txt $out/
      '';
    };

  # Run Go unit tests with coverage for the apis module.
  testAPIs =
    { version }:
    pkgs.buildGoApplication {
      pname = "crossplane-apis-test";
      inherit version;
      src = "${self}/apis";
      pwd = "${self}/apis";
      modules = "${self}/apis/gomod2nix.toml";

      CGO_ENABLED = "0";

      dontBuild = true;

      checkPhase = ''
        runHook preCheck
        export HOME=$TMPDIR
        go test -covermode=count -coverprofile=coverage.txt ./...
        runHook postCheck
      '';

      installPhase = ''
        mkdir -p $out
        cp coverage.txt $out/
      '';
    };

  # Run golangci-lint (without --fix, since source is read-only)
  goLint =
    { version }:
    pkgs.buildGoApplication {
      pname = "crossplane-go-lint";
      inherit version;
      src = self;
      pwd = self;
      modules = ../gomod2nix.toml;

      CGO_ENABLED = "0";

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

  # Run golangci-lint (without --fix, since source is read-only) for the apis module.
  goLintAPIs =
    { version }:
    pkgs.buildGoApplication {
      pname = "crossplane-apis-go-lint";
      inherit version;
      src = "${self}/apis";
      pwd = "${self}/apis";
      modules = "${self}/apis/gomod2nix.toml";

      CGO_ENABLED = "0";

      nativeBuildInputs = [ pkgs.golangci-lint ];

      dontBuild = true;

      checkPhase = ''
        runHook preCheck
        export HOME=$TMPDIR
        export GOLANGCI_LINT_CACHE=$TMPDIR/.cache/golangci-lint
        golangci-lint run --config=${self}/.golangci.yml
        runHook postCheck
      '';

      installPhase = ''
        mkdir -p $out
        touch $out/.lint-passed
      '';
    };

  # Run Helm linter
  helmLint =
    _:
    pkgs.runCommand "crossplane-helm-lint"
      {
        nativeBuildInputs = [ pkgs.kubernetes-helm ];
      }
      ''
        helm lint ${self}/cluster/charts/crossplane
        mkdir -p $out
        touch $out/.lint-passed
      '';

  # Verify generated code matches committed code
  generate =
    { version }:
    pkgs.buildGoApplication {
      pname = "crossplane-generate-check";
      inherit version;
      src = self;
      pwd = self;
      modules = ../gomod2nix.toml;

      CGO_ENABLED = "0";

      nativeBuildInputs = [
        pkgs.kubectl
        pkgs.helm-docs
        pkgs.buf
        pkgs.goverter
        pkgs.protoc-gen-go
        pkgs.protoc-gen-go-grpc
        pkgs.kubernetes-controller-tools
      ];

      dontBuild = true;

      checkPhase = ''
        runHook preCheck
        export HOME=$TMPDIR

        echo "Running go generate..."
        go generate -tags generate .

        echo "Generating Helm chart docs..."
        helm-docs --chart-search-root=cluster/charts

        echo "Comparing against committed source..."
        if ! diff -rq apis ${self}/apis > /dev/null 2>&1 || \
           ! diff -rq internal ${self}/internal > /dev/null 2>&1 || \
           ! diff -rq proto ${self}/proto > /dev/null 2>&1 || \
           ! diff -rq cluster/crds ${self}/cluster/crds > /dev/null 2>&1 || \
           ! diff -rq cluster/webhookconfigurations ${self}/cluster/webhookconfigurations > /dev/null 2>&1 || \
           ! diff -rq cluster/charts ${self}/cluster/charts > /dev/null 2>&1; then
          echo "ERROR: Generated code is out of date. Run 'nix run .#generate' and commit."
          exit 1
        fi

        runHook postCheck
      '';

      installPhase = ''
        mkdir -p $out
        touch $out/.generate-passed
      '';
    };

  # Verify generated code matches committed code for the apis module.
  generateAPIs =
    { version }:
    pkgs.buildGoApplication {
      pname = "crossplane-apis-generate-check";
      inherit version;
      src = "${self}/apis";
      pwd = "${self}/apis";
      modules = "${self}/apis/gomod2nix.toml";

      CGO_ENABLED = "0";

      nativeBuildInputs = [
        pkgs.kubectl
        pkgs.helm-docs
        pkgs.goverter
        pkgs.kubernetes-controller-tools
      ];

      dontBuild = true;

      checkPhase = ''
        runHook preCheck
        export HOME=$TMPDIR

        # cluster/webhookconfigurations contains some non-generated files. Copy
        # the existing version into our build context so we can detect changes
        # from generate, considering the manually populated bits.
        mkdir ../cluster
        cp -R ${self}/cluster/webhookconfigurations ../cluster/

        echo "Running go generate..."
        go generate -tags generate .

        echo "Patching CRDs..."
        kubectl patch --local --type=json \
          --patch-file ${self}/cluster/crd-patches/pkg.crossplane.io_deploymentruntimeconfigs.yaml \
          --filename ../cluster/crds/pkg.crossplane.io_deploymentruntimeconfigs.yaml \
          --output=yaml > /tmp/patched.yaml \
          && mv /tmp/patched.yaml ../cluster/crds/pkg.crossplane.io_deploymentruntimeconfigs.yaml

        echo "Comparing against committed source..."
        if ! diff -rq --exclude vendor . ${self}/apis > /dev/null 2>&1 || \
           ! diff -rq ../cluster/crds ${self}/cluster/crds > /dev/null 2>&1 || \
           ! diff -rq ../cluster/webhookconfigurations ${self}/cluster/webhookconfigurations > /dev/null 2>&1; then
          echo "ERROR: Generated code is out of date. Run 'nix run .#generate' and commit."
          exit 1
        fi

        runHook postCheck
      '';

      installPhase = ''
        mkdir -p $out
        touch $out/.generate-passed
      '';
    };

  # Run shell linters (shellcheck, shfmt)
  shellLint =
    _:
    pkgs.runCommand "crossplane-shell-lint"
      {
        nativeBuildInputs = [
          pkgs.findutils
          pkgs.shellcheck
          pkgs.shfmt
        ];
      }
      ''
        cd ${self}
        find . -name '*.sh' -type f | while read -r script; do
          shellcheck "$script"
          shfmt -d "$script"
        done
        mkdir -p $out
        touch $out/.shell-lint-passed
      '';

  # Run Nix linters (statix, deadnix, nixfmt)
  nixLint =
    _:
    pkgs.runCommand "crossplane-nix-lint"
      {
        nativeBuildInputs = [
          pkgs.statix
          pkgs.deadnix
          pkgs.nixfmt-rfc-style
        ];
      }
      ''
        statix check ${self}
        deadnix --fail ${self}/flake.nix ${self}/nix
        nixfmt --check ${self}/flake.nix ${self}/nix/*.nix
        mkdir -p $out
        touch $out/.nix-lint-passed
      '';
}
