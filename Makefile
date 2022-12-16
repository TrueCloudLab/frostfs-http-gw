#!/usr/bin/make -f

REPO ?= $(shell go list -m)
VERSION ?= $(shell git describe --tags --match "v*" --dirty --always --abbrev=8 2>/dev/null || cat VERSION 2>/dev/null || echo "develop")
GO_VERSION ?= 1.19
LINT_VERSION ?= 1.49.0
BUILD ?= $(shell date -u --iso=seconds)

HUB_IMAGE ?= truecloudlab/frostfs-http-gw
HUB_TAG ?= "$(shell echo ${VERSION} | sed 's/^v//')"

# List of binaries to build. For now just one.
BINDIR = bin
DIRS = $(BINDIR)
BINS = $(BINDIR)/frostfs-http-gw

.PHONY: all $(BINS) $(DIRS) dep docker/ test cover fmt image image-push dirty-image lint docker/lint version clean

# .deb package versioning
OS_RELEASE = $(shell lsb_release -cs)
PKG_VERSION ?= $(shell echo $(VERSION) | sed "s/^v//" | \
			sed -E "s/(.*)-(g[a-fA-F0-9]{6,8})(.*)/\1\3~\2/" | \
			sed "s/-/~/")-${OS_RELEASE}
.PHONY: debpackage debclean	

# Make all binaries
all: $(BINS)

$(BINS): $(DIRS) dep
	@echo "⇒ Build $@"
	CGO_ENABLED=0 \
	go build -v -trimpath \
	-ldflags "-X main.Version=$(VERSION)" \
	-o $@ ./

$(DIRS):
	@echo "⇒ Ensure dir: $@"
	@mkdir -p $@

# Pull go dependencies
dep:
	@printf "⇒ Download requirements: "
	@CGO_ENABLED=0 \
	go mod download && echo OK
	@printf "⇒ Tidy requirements: "
	@CGO_ENABLED=0 \
	go mod tidy -v && echo OK

# Run `make %` in Golang container, for more information run `make help.docker/%`
docker/%:
	$(if $(filter $*,all $(BINS)), \
		@echo "=> Running 'make $*' in clean Docker environment" && \
		docker run --rm -t \
		  -v `pwd`:/src \
		  -w /src \
		  -u `stat -c "%u:%g" .` \
		  --env HOME=/src \
		  golang:$(GO_VERSION) make $*,\
	  	@echo "supported docker targets: all $(BINS) lint")

# Run tests
test:
	@go test ./... -cover

# Run tests with race detection and produce coverage output
cover:
	@go test -v -race ./... -coverprofile=coverage.txt -covermode=atomic
	@go tool cover -html=coverage.txt -o coverage.html

# Reformat code
fmt:
	@echo "⇒ Processing gofmt check"
	@gofmt -s -w ./

# Build clean Docker image
image:
	@echo "⇒ Build FrostFS HTTP Gateway docker image "
	@docker build \
		--build-arg REPO=$(REPO) \
		--build-arg VERSION=$(VERSION) \
		--rm \
		-f Dockerfile \
		-t $(HUB_IMAGE):$(HUB_TAG) .

# Push Docker image to the hub
image-push:
	@echo "⇒ Publish image"
	@docker push $(HUB_IMAGE):$(HUB_TAG)

# Build dirty Docker image
dirty-image:
	@echo "⇒ Build FrostFS HTTP Gateway dirty docker image "
	@docker build \
		--build-arg REPO=$(REPO) \
		--build-arg VERSION=$(VERSION) \
		--rm \
		-f Dockerfile.dirty \
		-t $(HUB_IMAGE)-dirty:$(HUB_TAG) .

# Run linters
lint:
	@golangci-lint --timeout=5m run

# Run linters in Docker
docker/lint:
	docker run --rm -it \
	-v `pwd`:/src \
	-u `stat -c "%u:%g" .` \
	--env HOME=/src \
	golangci/golangci-lint:v$(LINT_VERSION) bash -c 'cd /src/ && make lint'

# Print version
version:
	@echo $(VERSION)

# Clean up
clean:
	rm -rf vendor
	rm -rf $(BINDIR)

# Package for Debian
debpackage:
	dch --package frostfs-http-gw \
			--controlmaint \
			--newversion $(PKG_VERSION) \
			--distribution $(OS_RELEASE) \
			"Please see CHANGELOG.md for code changes for $(VERSION)"
	dpkg-buildpackage --no-sign -b

debclean:
	dh clean

include help.mk
