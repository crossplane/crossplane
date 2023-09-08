# Copyright 2016 The Upbound Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

ifeq ($(HELM_CHARTS),)
$(error the variable HELM_CHARTS must be set prior to including helm.mk)
endif

# the base url where helm charts are published
ifeq ($(HELM_BASE_URL),)
$(error the variable HELM_BASE_URL must be set prior to including helm.mk)
endif

# the s3 bucket where helm charts are published
ifeq ($(HELM_S3_BUCKET),)
$(error the variable HELM_S3_BUCKET must be set prior to including helm.mk)
endif

# the charts directory
HELM_CHARTS_DIR ?= $(ROOT_DIR)/cluster/charts

# the charts output directory
HELM_OUTPUT_DIR ?= $(OUTPUT_DIR)/charts

# the helm index file
HELM_INDEX := $(HELM_OUTPUT_DIR)/index.yaml

HELM_CHART_LINT_STRICT ?= true
ifeq ($(HELM_CHART_LINT_STRICT),true)
HELM_CHART_LINT_STRICT_ARG += --strict
endif

# helm home
HELM_HOME := $(abspath $(WORK_DIR)/helm)
export HELM_HOME

# https://helm.sh/docs/faq/#xdg-base-directory-support
ifeq ($(USE_HELM3),true)
HELM_CACHE_HOME = $(HELM_HOME)/cache
HELM_CONFIG_HOME = $(HELM_HOME)/config
HELM_DATA_HOME = $(HELM_HOME)/data
export HELM_CACHE_HOME
export HELM_CONFIG_HOME
export HELM_DATA_HOME
endif

# remove the leading `v` for helm chart versions
HELM_CHART_VERSION := $(VERSION:v%=%)

#Chart Museum variables
#MUSEUM_URL  ?= "https://helm.example.com/" - url for chart museum
#If the following variables are set HTTP basic auth will be used. More details https://github.com/helm/chartmuseum/blob/master/README.md#basic-auth
#MUSEUM_USER ?= "helm"
#MUSEUM_PASS ?= "changeme"

# ====================================================================================
# Helm Targets
$(HELM_HOME): $(HELM)
	@mkdir -p $(HELM_HOME)
	@if [ "$(USE_HELM3)" == "false" ]; then \
		$(HELM) init -c --stable-repo-url=https://charts.helm.sh/stable; \
	fi

$(HELM_OUTPUT_DIR):
	@mkdir -p $(HELM_OUTPUT_DIR)

define helm.chart
$(HELM_OUTPUT_DIR)/$(1)-$(HELM_CHART_VERSION).tgz: $(HELM_HOME) $(HELM_OUTPUT_DIR) $(shell find $(HELM_CHARTS_DIR)/$(1) -type f)
	@$(INFO) helm package $(1) $(HELM_CHART_VERSION)
	@if [ "$(USE_HELM3)" == "false" ]; then \
		$(HELM) package --version $(HELM_CHART_VERSION) --app-version $(HELM_CHART_VERSION) --save=false -d $(HELM_OUTPUT_DIR) $(abspath $(HELM_CHARTS_DIR)/$(1)); \
	else \
		$(HELM) package --version $(HELM_CHART_VERSION) --app-version $(HELM_CHART_VERSION) -d $(HELM_OUTPUT_DIR) $(abspath $(HELM_CHARTS_DIR)/$(1)); \
	fi
	@$(OK) helm package $(1) $(HELM_CHART_VERSION)

helm.prepare.$(1): $(HELM_HOME)
	@cp -f $(HELM_CHARTS_DIR)/$(1)/values.yaml.tmpl $(HELM_CHARTS_DIR)/$(1)/values.yaml
	@cd $(HELM_CHARTS_DIR)/$(1) && $(SED_CMD) 's|%%VERSION%%|$(VERSION)|g' values.yaml

helm.prepare: helm.prepare.$(1)

helm.lint.$(1): $(HELM_HOME) helm.prepare.$(1)
	@rm -rf $(abspath $(HELM_CHARTS_DIR)/$(1)/charts)
	@$(HELM) dependency update $(abspath $(HELM_CHARTS_DIR)/$(1))
	@$(HELM) lint $(abspath $(HELM_CHARTS_DIR)/$(1)) $(HELM_CHART_LINT_ARGS_$(1)) $(HELM_CHART_LINT_STRICT_ARG)

helm.lint: helm.lint.$(1)

helm.dep.$(1): $(HELM_HOME)
	@$(INFO) helm dep $(1) $(HELM_CHART_VERSION)
	@$(HELM) dependency update $(abspath $(HELM_CHARTS_DIR)/$(1))
	@$(OK) helm dep $(1) $(HELM_CHART_VERSION)

helm.dep: helm.dep.$(1)

$(HELM_INDEX): $(HELM_OUTPUT_DIR)/$(1)-$(HELM_CHART_VERSION).tgz
endef
$(foreach p,$(HELM_CHARTS),$(eval $(call helm.chart,$(p))))

$(HELM_INDEX): $(HELM_HOME) $(HELM_OUTPUT_DIR)
	@$(INFO) helm index
	@$(HELM) repo index $(HELM_OUTPUT_DIR)
	@$(OK) helm index

helm.build: $(HELM_INDEX)

helm.clean:
	@rm -fr $(HELM_OUTPUT_DIR)

helm.env: $(HELM)
	@$(HELM) env

# ====================================================================================
# helm

HELM_TEMP := $(shell mktemp -d)
HELM_URL := $(HELM_BASE_URL)/$(CHANNEL)

helm.promote: $(HELM_HOME)
	@$(INFO) promoting helm charts
#	copy existing charts to a temp dir, the combine with new charts, reindex, and upload
	@$(S3_SYNC) s3://$(HELM_S3_BUCKET)/$(CHANNEL) $(HELM_TEMP)
	@if [ "$(S3_BUCKET)" != "" ]; then \
		$(S3_SYNC) s3://$(S3_BUCKET)/build/$(BRANCH_NAME)/$(VERSION)/charts $(HELM_TEMP); \
	fi
	@$(HELM) repo index --url $(HELM_URL) $(HELM_TEMP)
	@$(S3_SYNC_DEL) $(HELM_TEMP) s3://$(HELM_S3_BUCKET)/$(CHANNEL)
# 	re-upload index.yaml setting cache-control to ensure the file is not cached by http clients
	@$(S3_CP) --cache-control "private, max-age=0, no-transform" $(HELM_TEMP)/index.yaml s3://$(HELM_S3_BUCKET)/$(CHANNEL)/index.yaml
	@rm -fr $(HELM_TEMP)
	@$(OK) promoting helm charts

define museum.upload
helm.museum.$(1):
ifdef MUSEUM_URL
	@$(INFO) pushing helm charts $(1) to chart museum $(MUSEUM_URL)
ifneq ($(MUSEUM_USER)$(MUSEUM_PASS),"")
	@$(INFO) curl -u $(MUSEUM_USER):$(MUSEUM_PASS) --data-binary '@$(HELM_OUTPUT_DIR)/$(1)-$(HELM_CHART_VERSION).tgz' $(MUSEUM_URL)/api/charts
	@curl -u $(MUSEUM_USER):$(MUSEUM_PASS) --data-binary '@$(HELM_OUTPUT_DIR)/$(1)-$(HELM_CHART_VERSION).tgz' $(MUSEUM_URL)/api/charts
else
	@$(INFO) curl --data-binary '@$(HELM_OUTPUT_DIR)/$(1)-$(HELM_CHART_VERSION).tgz' $(MUSEUM_URL)/api/charts
	@curl --data-binary '@$(HELM_OUTPUT_DIR)/$(1)-$(HELM_CHART_VERSION).tgz' $(MUSEUM_URL)/api/charts
endif
	@$(OK) pushing helm charts to chart museum
endif

helm.museum: helm.museum.$(1)
endef
$(foreach p,$(HELM_CHARTS),$(eval $(call museum.upload,$(p))))

# ====================================================================================
# Common Targets

build.init: helm.prepare helm.lint
build.check: helm.dep
build.artifacts: helm.build
clean: helm.clean
lint: helm.lint
promote.artifacts: helm.promote helm.museum

# ====================================================================================
# Special Targets

dep: helm.dep

define HELM_HELPTEXT
Helm Targets:
    dep          Build and publish final releasable artifacts

endef
export HELM_HELPTEXT

helm.help:
	@echo "$$HELM_HELPTEXT"

help-special: helm.help

