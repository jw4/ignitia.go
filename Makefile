NAME=ignitia
IMAGE=docker.w.jw4.us/$(NAME)

ifeq ($(BUILD_VERSION),)
	BUILD_VERSION := $(shell git describe --dirty --first-parent --always --tags)
endif

.PHONY: all
all: image

.PHONY: local
local:
	docker build \
		--build-arg GOPROXY \
		-t $(IMAGE):$(BUILD_VERSION) \
		-t $(IMAGE):latest \
		.

.PHONY: image
image:
	docker buildx build \
		--build-arg GOPROXY \
		-t $(IMAGE):$(BUILD_VERSION) \
		-t $(IMAGE):latest \
		--platform linux/amd64,linux/arm64,linux/arm/v7 \
		.

.PHONY: push
push: image
	docker buildx build \
		--build-arg GOPROXY \
		-t $(IMAGE):$(BUILD_VERSION) \
		-t $(IMAGE):latest \
		--platform linux/amd64,linux/arm64,linux/arm/v7 \
		--push \
		.

