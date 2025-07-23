# Project Setup
PROJECT_NAME := provider-minio
PROJECT_REPO := github.com/vshn/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64
-include build/makelib/common.mk

# Setup Output
-include build/makelib/output.mk

# Setup Go
NPROCS ?= 1
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))
GO_STATIC_PACKAGES = $(GO_PROJECT)
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += internal apis
GO111MODULE = on
-include build/makelib/golang.mk

# Setup Kubernetes tools
UP_VERSION = v0.28.0
UP_CHANNEL = stable
UPTEST_VERSION = v0.11.1
-include build/makelib/k8s_tools.mk

# Setup Images
IMAGES = provider-minio
-include build/makelib/imagelight.mk

# Setup XPKG
XPKG_REG_ORGS ?= harbor.golder.lan/library
# NOTE: skip promoting on xpkg.upbound.io as channel tags are inferred.
XPKG_REG_ORGS_NO_PROMOTE ?= harbor.golder.lan/library
XPKGS = provider-minio
-include build/makelib/xpkg.mk

# NOTE: we force image building to happen prior to xpkg build so that we ensure
# image is present in daemon.
xpkg.build.provider-minio: do.build.images

# Setup Package Metadata
export CROSSPLANE_VERSION := $(shell go list -m -f '{{.Version}}' github.com/crossplane/crossplane)
-include build/makelib/local.xpkg.mk
-include build/makelib/controlplane.mk

# Targets

# run `make submodules` after cloning the repository for the first time.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

# Update the submodules, such as the common build scripts.
submodules.update:
	@git submodule update --remote --merge

# Install CRDs into a cluster
install: reviewable
	@$(INFO) installing CRDs into cluster
	@kubectl apply -f $(CRD_DIR)

# Uninstall CRDs from a cluster
uninstall:
	@$(INFO) uninstalling CRDs from cluster  
	@kubectl delete -f $(CRD_DIR)

# Integration tests
integration-test: generate
	@$(INFO) Running integration tests...
	@INTEGRATION_TESTS=true $(GO) test -v ./test/integration/...

.PHONY: submodules submodules.update install uninstall integration-test