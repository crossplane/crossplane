# ====================================================================================
# Setup Project

PROJECT_NAME := crossplane
PROJECT_REPO := github.com/crossplane/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64 linux_arm linux_ppc64le darwin_amd64 darwin_arm64 windows_amd64
# -include will silently skip missing files, which allows us
# to load those files with a target in the Makefile. If only
# "include" was used, the make command would fail and refuse
# to run a target until the include commands succeeded.
-include build/makelib/common.mk

# ====================================================================================
# Setup Output

S3_BUCKET ?= crossplane.releases
-include build/makelib/output.mk

# ====================================================================================
# Setup Go

# Set a sane default so that the nprocs calculation below is less noisy on the initial
# loading of this file
NPROCS ?= 1

# each of our test suites starts a kube-apiserver and running many test suites in
# parallel can lead to high CPU utilization. by default we reduce the parallelism
# to half the number of CPU cores.
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))

GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/crossplane $(GO_PROJECT)/cmd/crank $(GO_PROJECT)/cmd/xfn
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.version=$(VERSION)
GO_SUBDIRS += cmd internal apis
GO111MODULE = on
GOLANGCILINT_VERSION = 1.52.2
-include build/makelib/golang.mk

# ====================================================================================
# Setup Kubernetes tools

USE_HELM3 = true
HELM3_VERSION = v3.6.3
-include build/makelib/k8s_tools.mk

# ====================================================================================
# Setup Helm

HELM_BASE_URL = https://charts.crossplane.io
HELM_S3_BUCKET = crossplane.charts
HELM_CHARTS = crossplane
HELM_CHART_LINT_ARGS_crossplane = --set nameOverride='',imagePullSecrets=''
HELM_DOCS_ENABLED = true
HELM_VALUES_TEMPLATE_SKIPPED = true
-include build/makelib/helm.mk

# ====================================================================================
# Setup Images
# Due to the way that the shared build logic works, images should
# all be in folders at the same level (no additional levels of nesting).

REGISTRY_ORGS = docker.io/crossplane xpkg.upbound.io/crossplane
IMAGES = crossplane xfn
-include build/makelib/imagelight.mk

# ====================================================================================
# Targets

# run `make help` to see the targets and options

# We want submodules to be set up the first time `make` is run.
# We manage the build/ folder and its Makefiles as a submodule.
# The first time `make` is run, the includes of build/*.mk files will
# all fail, and this target will be run. The next time, the default as defined
# by the includes will be run instead.
fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

manifests:
	@$(WARN) Deprecated. Please run make generate instead.

CRD_DIR = cluster/crds

crds.clean:
	@$(INFO) cleaning generated CRDs
	@find $(CRD_DIR) -name '*.yaml' -exec sed -i.sed -e '1,1d' {} \; || $(FAIL)
	@find $(CRD_DIR) -name '*.yaml.sed' -delete || $(FAIL)
	@$(OK) cleaned generated CRDs

generate.run: gen-kustomize-crds gen-chart-license

gen-chart-license:
	@cp -f LICENSE cluster/charts/crossplane/LICENSE

generate.done: crds.clean

gen-kustomize-crds:
	@$(INFO) Adding all CRDs to Kustomize file for local development
	@rm cluster/kustomization.yaml
	@echo "# This kustomization can be used to remotely install all Crossplane CRDs" >> cluster/kustomization.yaml
	@echo "# by running kubectl apply -k https://github.com/crossplane/crossplane//cluster?ref=master" >> cluster/kustomization.yaml
	@echo "resources:" >> cluster/kustomization.yaml
	@find $(CRD_DIR) -type f -name '*.yaml' | sort | \
		while read filename ;\
		do echo "- $${filename#*/}" >> cluster/kustomization.yaml \
		; done
	@$(OK) All CRDs added to Kustomize file for local development

# Generate a coverage report for cobertura applying exclusions on
# - generated file
cobertura:
	@cat $(GO_TEST_OUTPUT)/coverage.txt | \
		grep -v zz_generated.deepcopy | \
		$(GOCOVER_COBERTURA) > $(GO_TEST_OUTPUT)/cobertura-coverage.xml

# integration tests
e2e.run: test-integration

# Run integration tests.
test-integration: $(KIND) $(KUBECTL) $(HELM3)
	@$(INFO) running integration tests using kind $(KIND_VERSION)
	@$(ROOT_DIR)/cluster/local/integration_tests.sh || $(FAIL)
	@$(OK) integration tests passed

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

# Install CRDs into a cluster. This is for convenience.
install-crds: $(KUBECTL) reviewable
	$(KUBECTL) apply -f $(CRD_DIR)

# Uninstall CRDs from a cluster. This is for convenience.
uninstall-crds:
	$(KUBECTL) delete -f $(CRD_DIR)

# Compile e2e tests.
# TODO(hasheddan): integrate this functionality into build submodule. The build
# submodule currently distinguishes tests and integration tests, but it only
# builds tests and does not support passing build tags only to the test build.
# Note that we are not publishing the builds that come from this step.
e2e-tests-compile:
	@$(INFO) Checking that e2e tests compile
	@$(GO) test -c -o $(WORK_DIR)/e2e/$(PLATFORM)/apiextensions.test ./test/e2e/apiextensions --tags=e2e
	@$(GO) test -c -o $(WORK_DIR)/e2e/$(PLATFORM)/pkg.test ./test/e2e/pkg --tags=e2e
	@$(OK) Verified e2e tests compile

# Compile e2e tests for each platform.
build.code.platform: e2e-tests-compile

# NOTE(hasheddan): the build submodule currently overrides XDG_CACHE_HOME in
# order to force the Helm 3 to use the .work/helm directory. This causes Go on
# Linux machines to use that directory as the build cache as well. We should
# adjust this behavior in the build submodule because it is also causing Linux
# users to duplicate their build cache, but for now we just make it easier to
# identify its location in CI so that we cache between builds.
go.cachedir:
	@go env GOCACHE

# This is for running out-of-cluster locally, and is for convenience. Running
# this make target will print out the command which was used. For more control,
# try running the binary directly with different arguments.
run: go.build
	@$(INFO) Running Crossplane locally out-of-cluster . . .
	@# To see other arguments that can be provided, run the command with --help instead
	$(GO_OUT_DIR)/$(PROJECT_NAME) core start --debug

.PHONY: manifests cobertura submodules fallthrough test-integration run install-crds uninstall-crds gen-kustomize-crds e2e-tests-compile

# ====================================================================================
# Special Targets

define CROSSPLANE_MAKE_HELP
Crossplane Targets:
    cobertura          Generate a coverage report for cobertura applying exclusions on generated files.
    submodules         Update the submodules, such as the common build scripts.
    run                Run crossplane locally, out-of-cluster. Useful for development.

endef
# The reason CROSSPLANE_MAKE_HELP is used instead of CROSSPLANE_HELP is because the crossplane
# binary will try to use CROSSPLANE_HELP if it is set, and this is for something different.
export CROSSPLANE_MAKE_HELP

crossplane.help:
	@echo "$$CROSSPLANE_MAKE_HELP"

help-special: crossplane.help

.PHONY: crossplane.help help-special