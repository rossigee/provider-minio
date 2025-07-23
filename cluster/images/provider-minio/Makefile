img.build:
	@cp ../../../_output/bin/linux_amd64/provider-minio .
	@docker build --platform $(IMAGE_PLATFORMS) -t $(IMAGE) .
	@rm provider-minio

img.done: img.build

.PHONY: img.build img.done
