.PHONY: all build build-linux install clean vet test fmt deps help

# labs — the Northlatch Labs agent harness. One binary, built from source.

BINARY_NAME=labs
BUILD_DIR=build
CMD_DIR=cmd/$(BINARY_NAME)

# Version: a tag if HEAD carries one, else the short commit. Override with VERSION=x.y.z.
VERSION_RAW:=$(strip $(shell git describe --tags --exact-match 2>/dev/null))
GIT_COMMIT_RAW:=$(strip $(shell git rev-parse --short=8 HEAD 2>/dev/null))
BUILD_TIME_RAW:=$(strip $(shell date -u +%FT%TZ))
VERSION?=$(if $(VERSION_RAW),$(VERSION_RAW),$(GIT_COMMIT_RAW))
GIT_COMMIT=$(if $(GIT_COMMIT_RAW),$(GIT_COMMIT_RAW),dev)
BUILD_TIME=$(if $(BUILD_TIME_RAW),$(BUILD_TIME_RAW),dev)
CONFIG_PKG=github.com/Northlatch-Labs-LLC/labs/pkg/config

GO?=go
GO_VERSION_RAW:=$(strip $(shell $(GO) env GOVERSION 2>/dev/null))
GO_VERSION=$(if $(GO_VERSION_RAW),$(firstword $(GO_VERSION_RAW)),unknown)
LDFLAGS=-X $(CONFIG_PKG).Version=$(VERSION) -X $(CONFIG_PKG).GitCommit=$(GIT_COMMIT) -X $(CONFIG_PKG).BuildTime=$(BUILD_TIME) -X $(CONFIG_PKG).GoVersion=$(GO_VERSION) -s -w

CGO_ENABLED?=0
GO_BUILD_TAGS?=goolm,stdjson
GOFLAGS?=-tags $(GO_BUILD_TAGS)
export CGO_ENABLED GOTOOLCHAIN=local

PLATFORM?=$(shell $(GO) env GOHOSTOS)
ARCH?=$(shell $(GO) env GOHOSTARCH)
BINARY_PATH=$(BUILD_DIR)/$(BINARY_NAME)-$(PLATFORM)-$(ARCH)

INSTALL_BIN_DIR?=$(HOME)/.local/bin

all: build

## build: build labs for this machine into build/labs
build:
	@echo "Building $(BINARY_NAME) $(VERSION) (git: $(GIT_COMMIT)) for $(PLATFORM)/$(ARCH)..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=$(PLATFORM) GOARCH=$(ARCH) $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_PATH) ./$(CMD_DIR)
	@ln -sf $(BINARY_NAME)-$(PLATFORM)-$(ARCH) $(BUILD_DIR)/$(BINARY_NAME)
	@echo "Build complete: $(BINARY_PATH)"

## build-linux: cross-build for the swarm hosts (linux/amd64) into build/labs-linux-amd64
build-linux:
	@$(MAKE) build PLATFORM=linux ARCH=amd64

## install: copy build/labs into $(INSTALL_BIN_DIR)
install: build
	@mkdir -p $(INSTALL_BIN_DIR)
	@cp $(BINARY_PATH) $(INSTALL_BIN_DIR)/$(BINARY_NAME).tmp && chmod +x $(INSTALL_BIN_DIR)/$(BINARY_NAME).tmp && mv -f $(INSTALL_BIN_DIR)/$(BINARY_NAME).tmp $(INSTALL_BIN_DIR)/$(BINARY_NAME)
	@echo "Installed $(INSTALL_BIN_DIR)/$(BINARY_NAME)"

## clean: remove build artifacts
clean:
	@rm -rf $(BUILD_DIR)

## vet: static analysis
vet:
	@$(GO) vet $(GOFLAGS) ./...

## test: run the Go tests
test:
	@$(GO) test $(GOFLAGS) ./...

## fmt: gofmt every Go file
fmt:
	@gofmt -l -w cmd pkg

## deps: download and verify modules
deps:
	@$(GO) mod download
	@$(GO) mod verify

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
