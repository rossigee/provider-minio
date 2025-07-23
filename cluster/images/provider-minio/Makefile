img.build:
	@docker build --platform $(IMAGE_PLATFORMS) -t $(IMAGE) .

img.done: img.build

.PHONY: img.build img.done