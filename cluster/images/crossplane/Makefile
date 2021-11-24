# ====================================================================================
# Setup Project

include ../../../build/makelib/common.mk

# ====================================================================================
#  Options

OSBASEIMAGE = gcr.io/distroless/static@sha256:d2b0ec3141031720cf5eedef3493b8e129bc91935a43b50562fbe5429878d96b
include ../../../build/makelib/imagelight.mk

# ====================================================================================
# Targets

img.build:
	@$(INFO) docker build $(IMAGE)
	@$(MAKE) BUILD_ARGS="--load" img.build.shared
	@$(OK) docker build $(IMAGE)

img.publish:
	@$(INFO) docker publish $(IMAGE)
	@$(MAKE) BUILD_ARGS="--push" img.build.shared
	@$(OK) docker publish $(IMAGE)

img.build.shared:
	@cp Dockerfile $(IMAGE_TEMP_DIR) || $(FAIL)
	@cp $(OUTPUT_DIR)/bin/$(OS)_$(ARCH)/crossplane $(IMAGE_TEMP_DIR) || $(FAIL)
	@cp -a ../../../cluster/crds $(IMAGE_TEMP_DIR) || $(FAIL)
	@cd $(IMAGE_TEMP_DIR) && $(SED_CMD) 's|BASEIMAGE|$(OSBASEIMAGE)|g' Dockerfile || $(FAIL)
	@docker buildx build $(BUILD_ARGS) \
		--platform $(IMAGE_PLATFORMS) \
		-t $(IMAGE) \
		$(IMAGE_TEMP_DIR) || $(FAIL)

img.promote:
	@$(INFO) docker promote $(FROM_IMAGE) to $(TO_IMAGE)
	@docker buildx imagetools create -t $(TO_IMAGE) $(FROM_IMAGE)
	@$(OK) docker promote $(FROM_IMAGE) to $(TO_IMAGE)
