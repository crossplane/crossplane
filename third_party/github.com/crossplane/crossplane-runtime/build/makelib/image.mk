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


# DEPRECATED: this module has been replaced by imagelight.mk and may be removed
# in the future.

# ====================================================================================
# Options

ifeq ($(origin IMAGE_DIR),undefined)
IMAGE_DIR := $(ROOT_DIR)/cluster/images
endif

ifeq ($(origin IMAGE_OUTPUT_DIR),undefined)
IMAGE_OUTPUT_DIR := $(OUTPUT_DIR)/images/$(PLATFORM)
endif

ifeq ($(origin IMAGE_TEMP_DIR),undefined)
IMAGE_TEMP_DIR := $(shell mktemp -d)
endif

# set the OS base image to alpine if in not defined. set your own image for each
# supported platform.
ifeq ($(origin OSBASEIMAGE),undefined)
OSBASE ?= alpine:3.13
ifeq ($(ARCH),$(filter $(ARCH),amd64 ppc64le))
OSBASEIMAGE = $(OSBASE)
else ifeq ($(ARCH),arm64)
OSBASEIMAGE = arm64v8/$(OSBASE)
else
$(error unsupported architecture $(ARCH))
endif
endif

# shasum is not available on all systems. In that case, fall back to sha256sum.
ifneq ($(shell type shasum 2>/dev/null),)
SHA256SUM := shasum -a 256
else
SHA256SUM := sha256sum
endif

# a registry that is scoped to the current build tree on this host. this enables
# us to have isolation between concurrent builds on the same system, as in the case
# of multiple working directories or on a CI system with multiple executors. All images
# tagged with this build registry can safely be untagged/removed at the end of the build.
ifeq ($(origin BUILD_REGISTRY), undefined)
BUILD_REGISTRY := build-$(shell echo $(HOSTNAME)-$(ROOT_DIR) | $(SHA256SUM) | cut -c1-8)
endif

MANIFEST_TOOL_VERSION=v1.0.3
MANIFEST_TOOL := $(TOOLS_HOST_DIR)/manifest-tool-$(MANIFEST_TOOL_VERSION)

# In order to reduce built time especially on jenkins, we maintain a cache
# of already built images. This cache contains images that can be used to help speed
# future docker build commands using docker's content addressable schemes.
# All cached images go in in a 'cache/' local registry and we follow an MRU caching
# policy -- keeping images that have been referenced around and evicting images
# that have to been referenced in a while (and according to a policy). Note we can
# not rely on the image's .CreatedAt date since docker only updates then when the
# image is created and not referenced. Instead we keep a date in the Tag.
CACHE_REGISTRY := cache

# prune images that are at least this many hours old
PRUNE_HOURS ?= 48

# prune keeps at least this many images regardless of how old they are
PRUNE_KEEP ?= 24

# don't actually prune just show what prune would do.
PRUNE_DRYRUN ?= 0

# the cached image format
CACHE_DATE_FORMAT := "%Y-%m-%d.%H%M%S"
CACHE_PRUNE_DATE := $(shell export TZ="UTC+$(PRUNE_HOURS)"; date +"$(CACHE_DATE_FORMAT)")
CACHE_TAG := $(shell date -u +"$(CACHE_DATE_FORMAT)")

REGISTRIES ?= $(DOCKER_REGISTRY)
IMAGE_ARCHS := $(subst linux_,,$(filter linux_%,$(PLATFORMS)))
IMAGE_PLATFORMS := $(subst _,/,$(subst $(SPACE),$(COMMA),$(filter linux_%,$(PLATFORMS))))

# if set to 1 docker image caching will not be used.
CACHEBUST ?= 0
ifeq ($(CACHEBUST),1)
BUILD_ARGS += --no-cache
endif

# if V=0 avoid showing verbose output from docker build
ifeq ($(V),0)
BUILD_ARGS ?= -q
endif

# if PULL=1 we will always check if there is a newer base image
PULL ?= 1
ifeq ($(PULL),1)
BUILD_BASE_ARGS += --pull
endif
BUILD_BASE_ARGS += $(BUILD_ARGS)
export PULL

# the version of tini to use
TINI_VERSION ?= v0.16.1

ifeq ($(HOSTOS),Linux)
SELF_CID := $(shell cat /proc/self/cgroup | grep docker | grep -o -E '[0-9a-f]{64}' | head -n 1)
endif

# =====================================================================================
# Image Targets

do.img.clean:
	@for i in $(CLEAN_IMAGES); do \
		if [ -n "$$(docker images -q $$i)" ]; then \
			for c in $$(docker ps -a -q --no-trunc --filter=ancestor=$$i); do \
				if [ "$$c" != "$(SELF_CID)" ]; then \
					echo stopping and removing container $${c} referencing image $$i; \
					docker stop $${c}; \
					docker rm $${c}; \
				fi; \
			done; \
			echo cleaning image $$i; \
			docker rmi $$i > /dev/null 2>&1 || true; \
		fi; \
	done

# this will clean everything for this build
img.clean:
	@$(INFO) cleaning images for $(BUILD_REGISTRY)
	@$(MAKE) do.img.clean CLEAN_IMAGES="$(shell docker images | grep -E '^$(BUILD_REGISTRY)/' | awk '{print $$1":"$$2}')"
	@$(OK) cleaning images for $(BUILD_REGISTRY)

img.done:
	@rm -fr $(IMAGE_TEMP_DIR)

img.cache:
	@for i in $(CACHE_IMAGES); do \
		IMGID=$$(docker images -q $$i); \
		if [ -n "$$IMGID" ]; then \
			echo === caching image $$i; \
			CACHE_IMAGE=$(CACHE_REGISTRY)/$${i#*/}; \
			docker tag $$i $${CACHE_IMAGE}:$(CACHE_TAG); \
			for r in $$(docker images --format "{{.ID}}#{{.Repository}}:{{.Tag}}" | grep $$IMGID | grep $(CACHE_REGISTRY)/ | grep -v $${CACHE_IMAGE}:$(CACHE_TAG)); do \
				docker rmi $${r#*#} > /dev/null 2>&1 || true; \
			done; \
		fi; \
	done

# prune removes old cached images
img.prune:
	@$(INFO) pruning images older than $(PRUNE_HOURS) keeping a minimum of $(PRUNE_KEEP) images
	@EXPIRED=$$(docker images --format "{{.Tag}}#{{.Repository}}:{{.Tag}}" \
		| grep -E '$(CACHE_REGISTRY)/' \
		| sort -r \
		| awk -v i=0 -v cd="$(CACHE_PRUNE_DATE)" -F  "#" '{if ($$1 <= cd && i >= $(PRUNE_KEEP)) print $$2; i++ }') &&\
	for i in $$EXPIRED; do \
		echo removing expired cache image $$i; \
		[ $(PRUNE_DRYRUN) = 1 ] || docker rmi $$i > /dev/null 2>&1 || true; \
	done
	@for i in $$(docker images -q -f dangling=true); do \
		echo removing dangling image $$i; \
		docker rmi $$i > /dev/null 2>&1 || true; \
	done
	@$(OK) pruning

debug.nuke:
	@for c in $$(docker ps -a -q --no-trunc); do \
		if [ "$$c" != "$(SELF_CID)" ]; then \
			echo stopping and removing container $${c}; \
			docker stop $${c}; \
			docker rm $${c}; \
		fi; \
	done
	@for i in $$(docker images -q); do \
		echo removing image $$i; \
		docker rmi -f $$i > /dev/null 2>&1; \
	done

# 1: registry 2: image, 3: arch
define repo.targets
img.release.build.$(1).$(2).$(3):
	@$(INFO) docker build $(1)/$(2)-$(3):$(VERSION)
	@docker tag $(BUILD_REGISTRY)/$(2)-$(3) $(1)/$(2)-$(3):$(VERSION) || $(FAIL)
	@# Save image as _output/images/linux_<arch>/<image>.tar.gz (no builds for darwin or windows)
	@mkdir -p $(OUTPUT_DIR)/images/linux_$(3) || $(FAIL)
	@docker save $(BUILD_REGISTRY)/$(2)-$(3) | gzip -c > $(OUTPUT_DIR)/images/linux_$(3)/$(2).tar.gz || $(FAIL)
	@$(OK) docker build $(1)/$(2)-$(3):$(VERSION)
img.release.build: img.release.build.$(1).$(2).$(3)

img.release.publish.$(1).$(2).$(3):
	@$(INFO) docker push $(1)/$(2)-$(3):$(VERSION)
	@docker push $(1)/$(2)-$(3):$(VERSION) || $(FAIL)
	@$(OK) docker push $(1)/$(2)-$(3):$(VERSION)
img.release.publish: img.release.publish.$(1).$(2).$(3)

img.release.promote.$(1).$(2).$(3):
	@$(INFO) docker promote $(1)/$(2)-$(3):$(VERSION) to $(1)/$(2)-$(3):$(CHANNEL)
	@docker pull $(1)/$(2)-$(3):$(VERSION) || $(FAIL)
	@[ "$(CHANNEL)" = "master" ] || docker tag $(1)/$(2)-$(3):$(VERSION) $(1)/$(2)-$(3):$(VERSION)-$(CHANNEL) || $(FAIL)
	@docker tag $(1)/$(2)-$(3):$(VERSION) $(1)/$(2)-$(3):$(CHANNEL) || $(FAIL)
	@[ "$(CHANNEL)" = "master" ] || docker push $(1)/$(2)-$(3):$(VERSION)-$(CHANNEL)
	@docker push $(1)/$(2)-$(3):$(CHANNEL) || $(FAIL)
	@$(OK) docker promote $(1)/$(2)-$(3):$(VERSION) to $(1)/$(2)-$(3):$(CHANNEL) || $(FAIL)
img.release.promote: img.release.promote.$(1).$(2).$(3)

img.release.clean.$(1).$(2).$(3):
	@[ -z "$$$$(docker images -q $(1)/$(2)-$(3):$(VERSION))" ] || docker rmi $(1)/$(2)-$(3):$(VERSION)
	@[ -z "$$$$(docker images -q $(1)/$(2)-$(3):$(VERSION)-$(CHANNEL))" ] || docker rmi $(1)/$(2)-$(3):$(VERSION)-$(CHANNEL)
	@[ -z "$$$$(docker images -q $(1)/$(2)-$(3):$(CHANNEL))" ] || docker rmi $(1)/$(2)-$(3):$(CHANNEL)
img.release.clean: img.release.clean.$(1).$(2).$(3)
endef
$(foreach r,$(REGISTRIES), $(foreach i,$(IMAGES), $(foreach a,$(IMAGE_ARCHS),$(eval $(call repo.targets,$(r),$(i),$(a))))))

img.release.manifest.publish.%: img.release.publish $(MANIFEST_TOOL)
	@$(MANIFEST_TOOL) push from-args --platforms $(IMAGE_PLATFORMS) --template $(DOCKER_REGISTRY)/$*-ARCH:$(VERSION) --target $(DOCKER_REGISTRY)/$*:$(VERSION) || $(FAIL)

img.release.manifest.promote.%: img.release.promote $(MANIFEST_TOOL)
	@[ "$(CHANNEL)" = "master" ] || $(MANIFEST_TOOL) push from-args --platforms $(IMAGE_PLATFORMS) --template $(DOCKER_REGISTRY)/$*-ARCH:$(VERSION) --target $(DOCKER_REGISTRY)/$*:$(VERSION)-$(CHANNEL) || $(FAIL)
	@$(MANIFEST_TOOL) push from-args --platforms $(IMAGE_PLATFORMS) --template $(DOCKER_REGISTRY)/$*-ARCH:$(VERSION) --target $(DOCKER_REGISTRY)/$*:$(CHANNEL) || $(FAIL)

# ====================================================================================
# Common Targets

# if IMAGES is defined then invoke and build each image identified
ifneq ($(IMAGES),)

ifeq ($(DOCKER_REGISTRY),)
$(error the variable DOCKER_REGISTRY must be set prior to including image.mk)
endif

do.build.image.%: ; @$(MAKE) -C $(IMAGE_DIR)/$* PLATFORM=$(PLATFORM)
do.build.images: $(foreach i,$(IMAGES), do.build.image.$(i)) ;
build.artifacts.platform: do.build.images
build.done: img.cache img.done
clean: img.clean img.release.clean

publish.init: img.release.build

# only publish images for main / master and release branches
# TODO(hasheddan): remove master and support overriding
ifneq ($(filter main master release-%,$(BRANCH_NAME)),)
publish.artifacts: $(addprefix img.release.manifest.publish.,$(IMAGES))
endif

promote.artifacts: $(addprefix img.release.manifest.promote.,$(IMAGES))

else

# otherwise we assume this .mk file is being included to build a single image

build.artifacts.platform: img.build
build.done: img.cache img.done
clean: img.clean

endif

# ====================================================================================
# Special Targets

prune: img.prune

define IMAGE_HELPTEXT
DEPRECATED: this module has been replaced by imagelight.mk and may be removed in the future. 

Image Targets:
    prune        Prune orphaned and cached images.

Image Options:
    PRUNE_HOURS  The number of hours from when an image is last used for it to be
                 considered a target for pruning. Default is 48 hrs.
    PRUNE_KEEP   The minimum number of cached images to keep. Default is 24 images.

endef
export IMAGE_HELPTEXT

img.help:
	@echo "$$IMAGE_HELPTEXT"

help-special: img.help

.PHONY: prune img.help

# ====================================================================================
# tools

$(MANIFEST_TOOL):
	@$(INFO) installing manifest-tool $(MANIFEST_TOOL_VERSION)
	@mkdir -p $(TOOLS_HOST_DIR) || $(FAIL)
	@curl -fsSL https://github.com/estesp/manifest-tool/releases/download/$(MANIFEST_TOOL_VERSION)/manifest-tool-$(HOSTOS)-$(SAFEHOSTARCH) > $@ || $(FAIL)
	@chmod +x $@ || $(FAIL)
	@$(OK) installing manifest-tool $(MANIFEST_TOOL_VERSION)
