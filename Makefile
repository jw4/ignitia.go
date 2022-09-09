NAME=ignitia
IMAGE=docker.w.jw4.us/$(NAME)

PLATFORMS="linux/amd64,linux/arm64,linux/arm/v7"
PLATFORMS="linux/amd64,linux/arm64"

ifeq ($(BUILD_VERSION),)
	BUILD_VERSION := $(shell git describe --dirty --first-parent --always --tags)
endif

.PHONY: all
all: image

.PHONY: run
run:
	go run \
		-tags=netgo \
		-ldflags '-s -w -extldflags "-static"' \
		-ldflags "-X main.version=$(BUILD_VERSION)" \
		./cmd/ignitia serve

.PHONY: image
image:
	docker build \
		--build-arg GOPROXY \
		--build-arg BUILD_VERSION=$(BUILD_VERSION) \
		-t $(IMAGE):$(BUILD_VERSION) \
		-t $(IMAGE):latest \
		.

.PHONY: run-image
run-image: image
	docker run --rm -it -p 8989:80 $(IMAGE):$(BUILD_VERSION)

.PHONY: push
push:
	docker buildx build \
		--build-arg GOPROXY \
		--build-arg BUILD_VERSION=$(BUILD_VERSION) \
		-t $(IMAGE):$(BUILD_VERSION) \
		-t $(IMAGE):latest \
		--platform $(PLATFORMS) \
		--push \
		.

