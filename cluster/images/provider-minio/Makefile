# ====================================================================================
# Setup Project

PLATFORMS := linux_amd64 linux_arm64
include ../../../build/makelib/common.mk

# ====================================================================================
# Options
IMAGE ?= $(BUILD_REGISTRY)/provider-minio:$(VERSION)
TARGETOS := $(word 1, $(subst _, ,$(PLATFORM)))
include ../../../build/makelib/imagelight.mk

# ====================================================================================
# Image Build
img.build:
	@$(INFO) docker build $(IMAGE)
	@$(MAKE) -C ../../.. go.build PLATFORM=$(PLATFORM)
	@$(MAKE) img.build.shared
	@$(OK) docker build $(IMAGE)

img.build.shared:
	@cp Dockerfile $(IMAGE_TEMP_DIR)
	@cp -r $(OUTPUT_DIR)/bin/ $(IMAGE_TEMP_DIR)/
	@cp -r $(ROOT_DIR)/package/crds $(IMAGE_TEMP_DIR)/
	@cp -r $(ROOT_DIR)/package/webhook $(IMAGE_TEMP_DIR)/
	@cp $(ROOT_DIR)/package/crossplane.yaml $(IMAGE_TEMP_DIR)/
	@docker build $(BUILD_ARGS) --build-arg TARGETOS=$(TARGETOS) --build-arg TARGETARCH=$(TARGETARCH) --build-arg VERSION=$(VERSION) -t $(IMAGE) $(IMAGE_TEMP_DIR)

# ====================================================================================
# Image Publish
img.publish: img.build
	@$(INFO) docker push $(IMAGE)
	docker push $(IMAGE)
	@$(OK) docker push $(IMAGE)
