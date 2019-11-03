# ====================================================================================
# Setup Project

PROJECT_NAME := crossplane
PROJECT_REPO := github.com/crossplaneio/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64
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

GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/crossplane
GO_LDFLAGS += -X $(GO_PROJECT)/pkg/version.Version=$(VERSION)
GO_SUBDIRS += cmd pkg apis
GO111MODULE = on
-include build/makelib/golang.mk

# ====================================================================================
# Setup Helm

HELM_BASE_URL = https://charts.crossplane.io
HELM_S3_BUCKET = crossplane.charts
HELM_CHARTS = crossplane
HELM_CHART_LINT_ARGS_crossplane = --set nameOverride='',imagePullSecrets=''
-include build/makelib/helm.mk

# ====================================================================================
# Setup Kubebuilder

CRD_DIR = cluster/charts/crossplane/templates/crds
API_DIR = ./apis/...

-include build/makelib/kubebuilder.mk

# ====================================================================================
# Setup Kubernetes tools

-include build/makelib/k8s_tools.mk

# ====================================================================================
# Setup Images

DOCKER_REGISTRY = crossplane
IMAGES = crossplane
-include build/makelib/image.mk

# ====================================================================================
# Setup Docs

SOURCE_DOCS_DIR = docs
DEST_DOCS_DIR = docs
DOCS_GIT_REPO = https://$(GIT_API_TOKEN)@github.com/crossplaneio/crossplaneio.github.io.git
-include build/makelib/docs.mk

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

go.test.unit: $(KUBEBUILDER)

# Generate manifests e.g. CRD, RBAC etc. locally for Stacks API types
# as it needs the custom "maxDescLen=0" option
manifests: vendor kubebuilder.manifests
	@$(INFO) Generating CRD manifests
	$(CONTROLLERGEN) crd:maxDescLen=0,trivialVersions=true paths=./apis/stacks/... output:dir=$(CRD_DIR)
# Add "helm.sh/hook: crd-install" and "helm.sh/hook-delete-policy: before-hook-creation" annotations for clusterstackinstalls and stackinstalls CRDs
	$(eval TMPDIR := $(shell mktemp -d))
	kustomize build cluster/charts -o $(TMPDIR)
	mv $(TMPDIR)/apiextensions.k8s.io_v1beta1_customresourcedefinition_clusterstackinstalls.stacks.crossplane.io.yaml $(CRD_DIR)/stacks.crossplane.io_clusterstackinstalls.yaml
	mv $(TMPDIR)/apiextensions.k8s.io_v1beta1_customresourcedefinition_stackinstalls.stacks.crossplane.io.yaml $(CRD_DIR)/stacks.crossplane.io_stackinstalls.yaml
	@$(OK) Generating CRD manifests

# Generate a coverage report for cobertura applying exclusions on
# - generated file
cobertura:
	@cat $(GO_TEST_OUTPUT)/coverage.txt | \
		grep -v zz_generated.deepcopy | \
		$(GOCOVER_COBERTURA) > $(GO_TEST_OUTPUT)/cobertura-coverage.xml

# Ensure a PR is ready for review.
reviewable: vendor generate manifests lint

# integration tests
e2e.run: test-integration

# Run integration tests.
test-integration: $(KIND) $(KUBECTL) $(HELM)
	@$(INFO) running integration tests using kind $(KIND_VERSION)
	@$(ROOT_DIR)/cluster/local/integration_tests.sh || $(FAIL)
	@$(OK) integration tests passed

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

# This is for running out-of-cluster locally, and is for convenience. Running
# this make target will print out the command which was used. For more control,
# try running the binary directly with different arguments.
run: go.build
	@$(INFO) Running Crossplane locally out-of-cluster . . .
	@# To see other arguments that can be provided, run the command with --help instead
	$(GO_OUT_DIR)/$(PROJECT_NAME) --debug

.PHONY: manifests cobertura reviewable submodules fallthrough test-integration run

# ====================================================================================
# Special Targets

define CROSSPLANE_MAKE_HELP
Crossplane Targets:
    manifests          Generate manifests e.g. CRD, RBAC etc.
    cobertura          Generate a coverage report for cobertura applying exclusions on generated files.
    reviewable         Ensure a PR is ready for review.
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

# target for resolving angryjet dependency
# TODO(soorena776): move this to golang.mk in build submodule
CROSSPLANETOOLS_ANGRYJET := $(TOOLS_HOST_DIR)/angryjet
export CROSSPLANETOOLS_ANGRYJET

$(CROSSPLANETOOLS_ANGRYJET):
	@$(INFO) installing Crossplane AngryJet
	@mkdir -p $(TOOLS_HOST_DIR)/tmp-angryjet || $(FAIL)
	@GO111MODULE=off GOPATH=$(TOOLS_HOST_DIR)/tmp-angryjet GOBIN=$(TOOLS_HOST_DIR) $(GOHOST) get github.com/crossplaneio/crossplane-tools/cmd/angryjet || rm -fr $(TOOLS_HOST_DIR)/tmp-angryjet|| $(FAIL)
	@rm -fr $(TOOLS_HOST_DIR)/tmp-angryjet
	@$(OK) installing Crossplane AngryJet

go.generate: $(CROSSPLANETOOLS_ANGRYJET)
