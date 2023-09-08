SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
SCRIPTS_DIR := $(SELF_DIR)/../scripts

KIND_CLUSTER_NAME ?= local-dev
DEPLOY_LOCAL_DIR ?= $(ROOT_DIR)/cluster/local
DEPLOY_LOCAL_POSTRENDER_WORKDIR := $(WORK_DIR)/local/post-render
DEPLOY_LOCAL_WORKDIR := $(WORK_DIR)/local/localdev
DEPLOY_LOCAL_CONFIG_DIR := $(DEPLOY_LOCAL_WORKDIR)/config
DEPLOY_LOCAL_KUBECONFIG := $(DEPLOY_LOCAL_WORKDIR)/kubeconfig
KIND_CONFIG_FILE := $(DEPLOY_LOCAL_WORKDIR)/kind.yaml
KUBECONFIG ?= $(HOME)/.kube/config

LOCALDEV_CLONE_WITH ?= ssh # or https
LOCALDEV_LOCAL_BUILD ?= true
LOCALDEV_PULL_LATEST ?= true

# HELM_HOME is defined in makelib/helm.mk, however, it is not possible to include makelib/helm.mk if
# repo has no helm charts where it fails with the variable HELM_CHARTS must be set prior to including helm.mk.
# We want to still use local dev tooling even the repo has no helm charts
# (e.g. deploying existing charts from other repositories).
ifndef HELM_HOME
HELM_HOME := $(abspath $(WORK_DIR)/helm)
XDG_DATA_HOME := $(HELM_HOME)
XDG_CONFIG_HOME := $(HELM_HOME)
XDG_CACHE_HOME := $(HELM_HOME)
export XDG_DATA_HOME
export XDG_CONFIG_HOME
export XDG_CACHE_HOME
$(HELM_HOME): $(HELM)
	@mkdir -p $(HELM_HOME)
endif

export BUILD_REGISTRIES=$(REGISTRIES)
ifndef REGISTRIES
	# To work with imagelight.mk
	export BUILD_REGISTRIES=$(REGISTRY_ORGS)
endif

export UP
export KIND
export KUBECTL
export KUSTOMIZE
export HELM
export HELM3
export USE_HELM3
export GOMPLATE
export ISTIO
export ISTIO_VERSION
export BUILD_REGISTRY
export ROOT_DIR
export SCRIPTS_DIR
export KIND_CLUSTER_NAME
export WORK_DIR
export LOCALDEV_INTEGRATION_CONFIG_REPO
export LOCAL_DEV_REPOS
export LOCALDEV_CLONE_WITH
export LOCALDEV_PULL_LATEST
export DEPLOY_LOCAL_DIR
export DEPLOY_LOCAL_POSTRENDER_WORKDIR
export DEPLOY_LOCAL_WORKDIR
export DEPLOY_LOCAL_CONFIG_DIR
export DEPLOY_LOCAL_KUBECONFIG
export KIND_CONFIG_FILE
export KUBECONFIG
export LOCALDEV_LOCAL_BUILD
export HELM_OUTPUT_DIR
export BUILD_HELM_CHART_VERSION=$(HELM_CHART_VERSION)
export BUILD_HELM_CHARTS_LIST=$(HELM_CHARTS)
export BUILD_IMAGES=$(IMAGES)
export BUILD_IMAGE_ARCHS=$(subst linux_,,$(filter linux_%,$(BUILD_PLATFORMS)))
export TARGETARCH

# Install gomplate
GOMPLATE_VERSION := 3.11.1
GOMPLATE := $(TOOLS_HOST_DIR)/gomplate-$(GOMPLATE_VERSION)

gomplate.buildvars:
	@echo GOMPLATE=$(GOMPLATE)

build.vars: gomplate.buildvars

$(GOMPLATE):
	@$(INFO) installing gomplate $(SAFEHOSTPLATFORM)
	@curl -fsSLo $(GOMPLATE) https://github.com/hairyhenderson/gomplate/releases/download/v$(GOMPLATE_VERSION)/gomplate_$(SAFEHOSTPLATFORM) || $(FAIL)
	@chmod +x $(GOMPLATE)
	@$(OK) installing gomplate $(SAFEHOSTPLATFORM)

kind.up: $(KIND)
	@$(INFO) kind up
	@$(KIND) get kubeconfig --name $(KIND_CLUSTER_NAME) >/dev/null 2>&1 || $(KIND) create cluster --name=$(KIND_CLUSTER_NAME) --config="$(KIND_CONFIG_FILE)" --kubeconfig="$(KUBECONFIG)"
	@$(KIND) get kubeconfig --name $(KIND_CLUSTER_NAME) > $(DEPLOY_LOCAL_KUBECONFIG)
	@$(OK) kind up

kind.down: $(KIND)
	@$(INFO) kind down
	@$(KIND) delete cluster --name=$(KIND_CLUSTER_NAME)
	@$(OK) kind down

kind.setcontext: $(KUBECTL) kind.up
	@$(KUBECTL) --kubeconfig $(KUBECONFIG) config use-context kind-$(KIND_CLUSTER_NAME)

kind.buildvars:
	@echo DEPLOY_LOCAL_KUBECONFIG=$(DEPLOY_LOCAL_KUBECONFIG)

build.vars: kind.buildvars

.PHONY: kind.up kind.down kind.setcontext kind.buildvars

local.helminit: $(KUBECTL) $(HELM) kind.setcontext
	@$(INFO) helm init
	@docker pull gcr.io/kubernetes-helm/tiller:$(HELM_VERSION)
	@$(KIND) load docker-image gcr.io/kubernetes-helm/tiller:$(HELM_VERSION) --name=$(KIND_CLUSTER_NAME)
	@$(KUBECTL) --kubeconfig $(KUBECONFIG) --namespace kube-system get serviceaccount tiller > /dev/null 2>&1 || $(KUBECTL) --kubeconfig $(KUBECONFIG) --namespace kube-system create serviceaccount tiller
	@$(KUBECTL) --kubeconfig $(KUBECONFIG) get clusterrolebinding tiller-cluster-rule > /dev/null 2>&1 || $(KUBECTL) --kubeconfig $(KUBECONFIG) create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
	@$(HELM) ls > /dev/null 2>&1 || $(HELM) init --kubeconfig $(KUBECONFIG) --service-account tiller --upgrade --wait
	@$(HELM) repo update
	@$(OK) helm init

-include $(DEPLOY_LOCAL_WORKDIR)/config.mk

local.prepare:
	@$(INFO) preparing local dev workdir
	@$(SCRIPTS_DIR)/localdev-prepare.sh || $(FAIL)
	@$(OK) preparing local dev workdir

local.clean:
	@$(INFO) cleaning local dev workdir
	@rm -rf $(WORK_DIR)/local || $(FAIL)
	@$(OK) cleaning local dev workdir

ifeq ($(USE_HELM3),true)
local.up: local.prepare kind.up $(HELM_HOME)
else
local.up: local.prepare kind.up local.helminit
endif

local.down: kind.down local.clean

local.deploy.%: local.prepare $(KUBECTL) $(KUSTOMIZE) $(HELM3) $(HELM_HOME) $(GOMPLATE) kind.setcontext
	@$(INFO) localdev deploy component: $*
	@$(eval PLATFORMS=$(BUILD_PLATFORMS))
	@$(SCRIPTS_DIR)/localdev-deploy-component.sh $* || $(FAIL)
	@$(OK) localdev deploy component: $*

local.remove.%: local.prepare $(KUBECTL) $(HELM3) $(HELM_HOME) $(GOMPLATE) kind.setcontext
	@$(INFO) localdev remove component: $*
	@$(SCRIPTS_DIR)/localdev-remove-component.sh $* || $(FAIL)
	@$(OK) localdev remove component: $*

local.scaffold:
	@$(INFO) localdev scaffold config
	@$(SCRIPTS_DIR)/localdev-scaffold.sh || $(FAIL)
	@$(OK) localdev scaffold config

.PHONY: local.helminit local.up local.deploy.% local.remove.%  local.scaffold

# ====================================================================================
# Special Targets

fmt: go.imports
fmt.simplify: go.fmt.simplify
imports: go.imports
imports.fix: go.imports.fix
vendor: go.vendor
vendor.check: go.vendor.check
vendor.update: go.vendor.update
vet: go.vet

define LOCAL_HELPTEXT
Local Targets:
    local.scaffold	scaffold a local development configuration
    local.up		stand up of a local development cluster with kind
    local.down		tear down local development cluster
    local.deploy.%	install/upgrade a local/external component, for example, local.deploy.crossplane
    local.remove.%	removes component, for example, local.remove.crossplane

endef
export LOCAL_HELPTEXT

local.help:
	@echo "$$LOCAL_HELPTEXT"

help-special: local.help

###
