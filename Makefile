# ====================================================================================
# Setup Project

PROJECT_NAME := crossplane
PROJECT_REPO := github.com/crossplane/$(PROJECT_NAME)

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
HELM_CHARTS = crossplane crossplane-types crossplane-controllers
HELM_CHART_LINT_ARGS_crossplane = --set nameOverride='',imagePullSecrets=''
-include build/makelib/helm.mk

# ====================================================================================
# Setup Kubernetes tools

-include build/makelib/k8s_tools.mk

# ====================================================================================
# Setup Images
# Due to the way that the shared build logic works, images should
# all be in folders at the same level (no additional levels of nesting).

DOCKER_REGISTRY = crossplane
IMAGES = crossplane
-include build/makelib/image.mk

# ====================================================================================
# Setup Docs

SOURCE_DOCS_DIR = docs
DEST_DOCS_DIR = docs
DOCS_GIT_REPO = https://$(GIT_API_TOKEN)@github.com/crossplane/crossplane.github.io.git
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

manifests:
	@$(WARN) Deprecated. Please run make generate instead.

generate: $(KUSTOMIZE) go.vendor go.generate manifests.prepare manifests.annotate
	@$(OK) Finished vendoring and generating


CROSSPLANE_TYPES_CHART_DIR = cluster/charts/crossplane-types
CROSSPLANE_CONTROLLERS_CHART_DIR = cluster/charts/crossplane-controllers
CROSSPLANE_CHART_DIR = cluster/charts/crossplane

CRD_DIR = $(CROSSPLANE_TYPES_CHART_DIR)/crds
CROSSPLANE_CHART_HELM2_CRD_DIR = $(CROSSPLANE_CHART_DIR)/templates/crds
CROSSPLANE_CHART_HELM3_CRD_DIR = $(CROSSPLANE_CHART_DIR)/crds

TYPE_MANIFESTS = $(wildcard $(CROSSPLANE_TYPES_CHART_DIR)/templates/*.yaml)
CONTROLLER_MANIFESTS = $(filter-out $(wildcard $(CROSSPLANE_CONTROLLERS_CHART_DIR)/templates/stack-manager-host-*.yaml), $(wildcard $(CROSSPLANE_CONTROLLERS_CHART_DIR)/templates/*.yaml))

# This target copies manifests in crossplane-controllers and crossplane-types chart into crossplane chart.
manifests.prepare:
	@$(INFO) Copying CRD manifests to Crossplane chart
	rm -r $(CROSSPLANE_CHART_HELM2_CRD_DIR)
	mkdir $(CROSSPLANE_CHART_HELM2_CRD_DIR)
	cp $(CRD_DIR)/* $(CROSSPLANE_CHART_HELM2_CRD_DIR)
	@$(OK) Copied CRD manifests to Crossplane chart
	@$(INFO) Copying controller manifests to Crossplane chart
	cp $(CONTROLLER_MANIFESTS) $(CROSSPLANE_CHART_DIR)/templates
	@$(OK) Copied controller manifests to Crossplane chart
	@$(INFO) Copying type manifests to Crossplane chart
	cp $(TYPE_MANIFESTS) $(CROSSPLANE_CHART_DIR)/templates
	@$(OK) Copied type manifests to Crossplane chart


# Add "helm.sh/hook: crd-install" and "helm.sh/hook-delete-policy:
# before-hook-creation" annotations for clusterpackageinstalls and packageinstalls
# CRDs. Since Crossplane helm chart contains both CRD and ClusterPackageInstall
# CRs, helm fails to install both together. One option was to use
# `post-install,post-update` hooks in CR to deploy it after CRDs are installed,
# but this didn't work reliably with "helm upgrade --install" command. Using
# "crd-install" hook is already suggested in helm best practices doc:
# https://helm.sh/docs/chart_best_practices/#method-2-crd-install-hooks and we
# verified that it works reliably for all use cases. The other hook for deletion
# policy is necessary to be able to redeploy helm chart after it is deleted
# since CRDs with "crd-install" hooks will not be deleted with "helm delete" and
# cause next "helm install" to fail.
# "helm.sh/hook: crd-install" was deprecated in helm3 and CRDs with this annotation are
# skipped. This results in the PackageInstall and ClusterPackageInstall CRDs not being
# installed in helm3 when they have that annotation.
# As a workaround, we first copy those CRDs under <chart>/crds directory which
# was introduced with helm3 and ignored in helm2, then afterwards apply the annotation to
# those CRDs under <chart>/templates/crds for helm2.
manifests.annotate:
	@$(INFO) Copying PackageInstall CRD manifests for helm3 compatibility
	rm -r $(CROSSPLANE_CHART_HELM3_CRD_DIR)
	mkdir -p $(CROSSPLANE_CHART_HELM3_CRD_DIR)
	cp $(CRD_DIR)/packages.crossplane.io_packageinstalls.yaml $(CROSSPLANE_CHART_HELM3_CRD_DIR)/packages.crossplane.io_packageinstalls.yaml
	cp $(CRD_DIR)/packages.crossplane.io_clusterpackageinstalls.yaml $(CROSSPLANE_CHART_HELM3_CRD_DIR)/packages.crossplane.io_clusterpackageinstalls.yaml
	@$(OK) Copied PackageInstall CRD manifests for helm3 compatibility
	@$(INFO) Annotating generated PackageInstall CRD manifests
	$(eval TMPDIR := $(shell mktemp -d))
	$(KUSTOMIZE) build cluster/charts -o $(TMPDIR)
	mv $(TMPDIR)/apiextensions.k8s.io_v1beta1_customresourcedefinition_clusterpackageinstalls.packages.crossplane.io.yaml $(CROSSPLANE_CHART_HELM2_CRD_DIR)/packages.crossplane.io_clusterpackageinstalls.yaml
	mv $(TMPDIR)/apiextensions.k8s.io_v1beta1_customresourcedefinition_packageinstalls.packages.crossplane.io.yaml $(CROSSPLANE_CHART_HELM2_CRD_DIR)/packages.crossplane.io_packageinstalls.yaml
	@$(OK) Annotated generated PackageInstall CRD manifests
	sed '1,7d' $(SOURCE_DOCS_DIR)/getting-started/install.md > $(CROSSPLANE_CHART_DIR)/README.md
	@$(OK) Copied and modified chart README.md from Crossplane docs

# Generate a coverage report for cobertura applying exclusions on
# - generated file
cobertura:
	@cat $(GO_TEST_OUTPUT)/coverage.txt | \
		grep -v zz_generated.deepcopy | \
		$(GOCOVER_COBERTURA) > $(GO_TEST_OUTPUT)/cobertura-coverage.xml

# Ensure a PR is ready for review.
reviewable: generate lint
	@go mod tidy

# Ensure branch is clean.
check-diff: reviewable
	@$(INFO) checking that branch is clean
	@test -z "$$(git status --porcelain)" || $(FAIL)
	@$(OK) branch is clean

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
