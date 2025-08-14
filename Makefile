# Project Setup
PROJECT_NAME := provider-minio
PROJECT_REPO := github.com/rossigee/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64
-include build/makelib/common.mk

# Setup Output
-include build/makelib/output.mk

# Setup Go
# Override golangci-lint version for modern Go support
GOLANGCILINT_VERSION ?= 2.4.0
NPROCS ?= 1
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/provider
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += operator apis
GO111MODULE = on
-include build/makelib/golang.mk

# Setup Kubernetes tools
UP_VERSION = v0.28.0
UP_CHANNEL = stable
UPTEST_VERSION = v0.11.1
-include build/makelib/k8s_tools.mk

# Setup Images
IMAGES = provider-minio
# Force registry override (can be overridden by make command arguments)
REGISTRY_ORGS = ghcr.io/rossigee
-include build/makelib/imagelight.mk

# Setup XPKG - Standardized registry configuration
# Primary registry: GitHub Container Registry under rossigee
XPKG_REG_ORGS ?= ghcr.io/rossigee
XPKG_REG_ORGS_NO_PROMOTE ?= ghcr.io/rossigee

# Optional registries (can be enabled via environment variables)
# Harbor publishing has been removed - using only ghcr.io/rossigee
# To enable Upbound: export ENABLE_UPBOUND_PUBLISH=true make publish XPKG_REG_ORGS=xpkg.upbound.io/crossplane-contrib
XPKGS = provider-minio
-include build/makelib/xpkg.mk

# NOTE: we force image building to happen prior to xpkg build so that we ensure
# image is present in daemon.
xpkg.build.provider-minio: do.build.images

# Setup Package Metadata
CROSSPLANE_VERSION = 1.19.0
-include build/makelib/local.xpkg.mk
-include build/makelib/controlplane.mk

# Setup Documentation
-include docs/antora-build.mk

# Targets

# run `make submodules` after cloning the repository for the first time.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

# NOTE: the build submodule currently overrides XDG_CACHE_HOME in order to
# force the Helm 3 to use the .work/helm directory. This causes Go on Linux
# machines to use that directory as the build cache as well. We should adjust
# this behavior in the build submodule because it is also causing Linux users
# to duplicate their build cache, but for now we just make it easier to identify
# its location in CI so that we cache between builds.
go.cachedir:
	@go env GOCACHE

# NOTE: we must ensure up is installed in tool cache prior to build as including the k8s_tools
# machinery prior to the xpkg machinery sets UP to point to tool cache.
build.init: $(UP)

# This is for running out-of-cluster locally, and is for convenience. Running
# this make target will print out the command which was used. For more control,
# try running the binary directly with different arguments.
run: go.build
	@$(INFO) Running Crossplane locally out-of-cluster . . .
	@# To see other arguments that can be provided, run the command with --help instead
	$(GO_OUT_DIR)/provider --debug

# NOTE: we ensure up is installed prior to running platform-specific packaging steps in xpkg.build.
xpkg.build: $(UP)

# Install CRDs into a cluster
install-crds: generate
	kubectl apply -f package/crds

# Uninstall CRDs from a cluster
uninstall-crds:
	kubectl delete -f package/crds

# Install samples into cluster
install-samples:
	kubectl apply -f ./samples/_secret.yaml
	yq ./samples/minio*.yaml | kubectl apply -f -

# Delete samples from cluster
delete-samples:
	-yq ./samples/*.yaml | kubectl delete --ignore-not-found --wait=false -f -

# Generate webhook certificates for out-of-cluster debugging
webhook-cert:
	mkdir -p .work/webhook
	openssl req -x509 -newkey rsa:4096 -nodes -keyout .work/webhook/tls.key -out .work/webhook/tls.crt -days 3650 -subj "/CN=host.docker.internal" -addext "subjectAltName = DNS:host.docker.internal"

# Setup webhook for debugging
webhook-debug: webhook-cert
	kubectl apply -f package/webhook
	cabundle=$$(cat .work/webhook/tls.crt | base64) && \
	HOSTIP=host.docker.internal && \
	kubectl get validatingwebhookconfigurations.admissionregistration.k8s.io validating-webhook-configuration -oyaml | \
	yq e "del(.webhooks[0].clientConfig.service) | .webhooks[0].clientConfig.caBundle |= \"$$cabundle\" | .webhooks[0].clientConfig.url |= \"https://$$HOSTIP:9443//validate-minio-crossplane-io-v1-bucket\"" - | \
	yq e "del(.webhooks[1].clientConfig.service) | .webhooks[1].clientConfig.caBundle |= \"$$cabundle\" | .webhooks[1].clientConfig.url |= \"https://$$HOSTIP:9443//validate-minio-crossplane-io-v1-policy\"" - | \
	yq e "del(.webhooks[2].clientConfig.service) | .webhooks[2].clientConfig.caBundle |= \"$$cabundle\" | .webhooks[2].clientConfig.url |= \"https://$$HOSTIP:9443//validate-minio-crossplane-io-v1-user\"" - | \
	kubectl apply -f -

.PHONY: submodules run install-crds uninstall-crds install-samples delete-samples webhook-cert webhook-debug
