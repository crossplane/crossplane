# ====================================================================================
# Setup Project

PROJECT_NAME := conductor
PROJECT_REPO := github.com/upbound/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64
include build/makelib/common.mk

# ====================================================================================
# Setup Output

S3_BUCKET ?= upbound.releases/conductor
include build/makelib/output.mk

# ====================================================================================
# Setup Go

GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/conductor
GO_LDFLAGS += -X $(GO_PROJECT)/pkg/version.Version=$(VERSION)
include build/makelib/golang.mk

# ====================================================================================
# Setup Helm

HELM_BASE_URL = https://charts.upbound.io
HELM_S3_BUCKET = upbound.charts
HELM_CHARTS = conductor
HELM_CHART_LINT_ARGS_conductor = --set nameOverride='',imagePullSecrets=''
include build/makelib/helm.mk

# ====================================================================================
# Setup Kubebuilder

include build/makelib/kubebuilder.mk

# ====================================================================================
# Setup Images

DOCKER_REGISTRY = upbound
IMAGES = conductor
include build/makelib/image.mk

# ====================================================================================
# Targets

# run `make help` to see the targets and options