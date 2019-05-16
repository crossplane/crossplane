# ====================================================================================
# Setup Project

PROJECT_NAME := crossplane
PROJECT_REPO := github.com/crossplaneio/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64
include build/makelib/common.mk

# ====================================================================================
# Setup Output

S3_BUCKET ?= crossplane.releases
include build/makelib/output.mk

# ====================================================================================
# Setup Go

# each of our test suites starts a kube-apiserver and running many test suites in
# parallel can lead to high CPU utilization. by default we reduce the parallelism
# to half the number of CPU cores.
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))

GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/crossplane
GO_LDFLAGS += -X $(GO_PROJECT)/pkg/version.Version=$(VERSION)
include build/makelib/golang.mk

# ====================================================================================
# Setup Helm

HELM_BASE_URL = https://charts.crossplane.io
HELM_S3_BUCKET = crossplane.charts
HELM_CHARTS = crossplane
HELM_CHART_LINT_ARGS_crossplane = --set nameOverride='',imagePullSecrets=''
include build/makelib/helm.mk

# ====================================================================================
# Setup Kubebuilder

include build/makelib/kubebuilder.mk

# ====================================================================================
# Setup Images

DOCKER_REGISTRY = crossplane
IMAGES = crossplane
include build/makelib/image.mk

# ====================================================================================
# Setup Docs

SOURCE_DOCS_DIR = docs
DEST_DOCS_DIR = docs
DOCS_GIT_REPO = https://$(GIT_API_TOKEN)@github.com/crossplaneio/crossplaneio.github.io.git
include build/makelib/docs.mk

# ====================================================================================
# Targets

# run `make help` to see the targets and options

go.test.unit: $(KUBEBUILDER)

# Generate manifests e.g. CRD, RBAC etc.
manifests: vendor
	@$(INFO) Generating CRD manifests
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go crd --output-dir cluster/charts/crossplane/crds --nested
	@$(OK) Generating CRD manifests

# Generate a coverage report for cobertura applying exclusions on
# - generated file
cobertura:
	@cat $(GO_TEST_OUTPUT)/coverage.txt | \
		grep -v zz_generated.deepcopy | \
		$(GOCOVER_COBERTURA) > $(GO_TEST_OUTPUT)/cobertura-coverage.xml

.PHONY: manifests cobertura
