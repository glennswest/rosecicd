VERSION ?= 0.1.0
REGISTRY ?= 192.168.200.2:5000
GHCR ?= ghcr.io/glennswest
GOFLAGS ?= -trimpath

.PHONY: all build build-controller build-builder image image-builder push clean

all: build

build: build-controller build-builder

build-controller:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -o bin/rosecicd ./cmd/rosecicd

build-builder:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -o bin/rosecicd-builder ./cmd/rosecicd-builder

image: build-controller
	buildah bud -f Dockerfile -t $(REGISTRY)/rosecicd:$(VERSION) -t $(REGISTRY)/rosecicd:edge .
	buildah push $(REGISTRY)/rosecicd:edge

image-builder: build-builder
	buildah bud -f Dockerfile.builder -t $(REGISTRY)/rosecicd-builder:$(VERSION) -t $(REGISTRY)/rosecicd-builder:edge .
	buildah push $(REGISTRY)/rosecicd-builder:edge

push: image image-builder
	buildah push $(GHCR)/rosecicd:$(VERSION)
	buildah push $(GHCR)/rosecicd-builder:$(VERSION)

clean:
	rm -rf bin/
