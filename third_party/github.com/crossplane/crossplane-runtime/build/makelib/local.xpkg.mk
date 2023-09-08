# Copyright 2022 The Upbound Authors. All rights reserved.
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

KIND_CLUSTER_NAME ?= local-dev
CROSSPLANE_NAMESPACE ?= crossplane-system

local.xpkg.init: $(KUBECTL)
	@$(INFO) patching Crossplane with dev sidecar
	@if ! $(KUBECTL) -n $(CROSSPLANE_NAMESPACE) get deployment crossplane -o jsonpath="{.spec.template.spec.containers[*].name}" | grep "dev" > /dev/null; then \
		$(KUBECTL) -n $(CROSSPLANE_NAMESPACE) patch deployment/crossplane --type='json' -p='[{"op":"add","path":"/spec/template/spec/containers/1","value":{"image":"alpine","name":"dev","command":["sleep","infinity"],"volumeMounts":[{"mountPath":"/tmp/cache","name":"package-cache"}]}},{"op":"add","path":"/spec/template/metadata/labels/patched","value":"true"}]'; \
		$(KUBECTL) -n $(CROSSPLANE_NAMESPACE) wait deploy crossplane --for condition=Available --timeout=60s; \
		$(KUBECTL) -n $(CROSSPLANE_NAMESPACE) wait pods -l app=crossplane,patched=true --for condition=Ready --timeout=60s; \
	fi
	@$(OK) patching Crossplane with dev sidecar

local.xpkg.sync: local.xpkg.init $(UP)
	@$(INFO) copying local xpkg cache to Crossplane pod
	@mkdir -p $(XPKG_OUTPUT_DIR)/cache
	@for pkg in $(XPKG_OUTPUT_DIR)/linux_*/*; do $(UP) xpkg xp-extract --from-xpkg $$pkg -o $(XPKG_OUTPUT_DIR)/cache/$$(basename $$pkg .xpkg).gz; done
	@XPPOD=$$($(KUBECTL) -n $(CROSSPLANE_NAMESPACE) get pod -l app=crossplane,patched=true -o jsonpath="{.items[0].metadata.name}"); \
		$(KUBECTL) -n $(CROSSPLANE_NAMESPACE) cp $(XPKG_OUTPUT_DIR)/cache -c dev $$XPPOD:/tmp
	@$(OK) copying local xpkg cache to Crossplane pod

local.xpkg.deploy.configuration.%: local.xpkg.sync
	@$(INFO) deploying configuration package $* $(VERSION)
	@echo '{"apiVersion":"pkg.crossplane.io/v1","kind":"Configuration","metadata":{"name":"$*"},"spec":{"package":"$*-$(VERSION).gz","packagePullPolicy":"Never"}}' | $(KUBECTL) apply -f -
	@$(OK) deploying configuration package $* $(VERSION)

local.xpkg.deploy.provider.%: $(KIND) local.xpkg.sync
	@$(INFO) deploying provider package $* $(VERSION)
	@$(KIND) load docker-image $(BUILD_REGISTRY)/$*-$(ARCH) -n $(KIND_CLUSTER_NAME)
	@echo '{"apiVersion":"pkg.crossplane.io/v1alpha1","kind":"ControllerConfig","metadata":{"name":"config"},"spec":{"args":["-d"],"image":"$(BUILD_REGISTRY)/$*-$(ARCH)"}}' | $(KUBECTL) apply -f -
	@echo '{"apiVersion":"pkg.crossplane.io/v1","kind":"Provider","metadata":{"name":"$*"},"spec":{"package":"$*-$(VERSION).gz","packagePullPolicy":"Never","controllerConfigRef":{"name":"config"}}}' | $(KUBECTL) apply -f -
	@$(OK) deploying provider package $* $(VERSION)
