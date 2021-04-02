# ====================================================================================
# Setup Project

PLATFORMS := linux_amd64 linux_arm64 linux_arm
include ../../../build/makelib/common.mk

# ====================================================================================
#  Options
IMAGE = $(BUILD_REGISTRY)/crossplane-$(ARCH)
OSBASEIMAGE = gcr.io/distroless/static
include ../../../build/makelib/image.mk

# ====================================================================================
# Targets

ifeq ($(PLATFORM),$(filter $(PLATFORM),darwin_amd64 windows_amd64)) 
$(info Skipping image build for $(PLATFORM))
img.build:
else
img.build:
	@$(INFO) docker build $(IMAGE)
	@cp Dockerfile $(IMAGE_TEMP_DIR) || $(FAIL)
	@cp $(OUTPUT_DIR)/bin/$(OS)_$(ARCH)/crossplane $(IMAGE_TEMP_DIR) || $(FAIL)
	@cd $(IMAGE_TEMP_DIR) && $(SED_CMD) 's|BASEIMAGE|$(OSBASEIMAGE)|g' Dockerfile || $(FAIL)
	@docker build $(BUILD_ARGS) \
		--platform linux/$(ARCH) \
		-t $(IMAGE) \
		$(IMAGE_TEMP_DIR) || $(FAIL)
	@$(OK) docker build $(IMAGE)
endif
