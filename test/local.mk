# Scratch directory for test state. The build submodule does not define
# kind_dir or go_bin, which is part of why this file had no working includer.
kind_dir ?= $(OUTPUT_DIR)/test
go_bin    ?= $(OUTPUT_DIR)/bin

# The build submodule invokes docker from PATH and does not define a DOCKER
# variable, so one is defined here for the Crossplane image side-load below.
DOCKER ?= docker

# The e2e suite gets its own kubeconfig rather than relying on whatever context
# the ambient KUBECONFIG happens to point at. A shared kubeconfig means a stray
# current-context can make `make test-e2e` install a provider into a real
# cluster.
KIND_KUBECONFIG ?= $(kind_dir)/kubeconfig

# Crossplane control plane for the end to end suite.
#
# The provider requires Crossplane >= v2.5.0 and upstream publishes no v2.5.0: the
# latest release is v2.4.2, charts.crossplane.io/stable tops out at v2.4.2, and
# neither docker.io, ghcr.io nor xpkg.crossplane.io carries a v2.5.0 or v2.5.0-rc.0
# image. This project publishes its own 2.5.0 control plane, so the suite installs
# that rather than building one. See crossplane-up below for why controlplane.up
# from the build submodule cannot be used.
#
# The published image at ghcr.io/rossigee/crossplane:v2.5.0 is a multi-arch manifest
# over linux/amd64, linux/arm64, linux/arm/v7 and linux/ppc64le, built from the
# rossigee/crossplane develop branch at e33ad34 with `buildVersion` pinned to
# v2.5.0 so the binary self-reports v2.5.0 and satisfies the provider's constraint.
#
# To build the control plane from source instead (requires Nix, which is not a
# normal developer tool for this repo), run `make crossplane-image`; that target
# clones the fork and produces a loadable image. CROSSPLANE_IMAGE_REPOSITORY and
# CROSSPLANE_IMAGE_TAG override the deployed image, for example:
#   make test-e2e CROSSPLANE_IMAGE_REPOSITORY=ghcr.io/rossigee/crossplane \
#                   CROSSPLANE_IMAGE_TAG=v2.5.0
CROSSPLANE_FORK_URL     ?= https://github.com/rossigee/crossplane.git
CROSSPLANE_FORK_BRANCH  ?= develop
CROSSPLANE_BUILD_VERSION ?= v2.5.0
CROSSPLANE_IMAGE_REPOSITORY ?= ghcr.io/rossigee/crossplane
CROSSPLANE_IMAGE_TAG        ?= $(CROSSPLANE_BUILD_VERSION)
CROSSPLANE_LOCAL_IMAGE      ?= crossplane-controlplane:$(CROSSPLANE_BUILD_VERSION)
CROSSPLANE_SRC_DIR          ?= $(kind_dir)/crossplane-src
# How long to wait for the crossplane and rbac-manager deployments to become
# available after the chart is installed. Generous because a cold node still
# has to schedule the pods, pull any remaining images and run crossplane-init
# before Core reports available.
CROSSPLANE_WAIT_TIMEOUT ?= 10m

# Build the control plane image from the fork's develop branch. Requires Nix,
# which is not a normal developer tool for this repo, so this is a separate
# target rather than part of test-e2e.
.PHONY: crossplane-image
crossplane-image: ## Build the Crossplane control plane from the fork's develop branch
	@$(INFO) building Crossplane $(CROSSPLANE_BUILD_VERSION) from $(CROSSPLANE_FORK_BRANCH)
	@command -v nix >/dev/null 2>&1 || { \
		echo "nix is required to build the Crossplane control plane from source."; \
		echo "Either install nix, or point the suite at an already published 2.5.0 image:"; \
		echo "  make test-e2e CROSSPLANE_IMAGE_REPOSITORY=<repo> CROSSPLANE_IMAGE_TAG=v2.5.0"; \
		exit 1; \
	}
	@test -d "$(CROSSPLANE_SRC_DIR)/.git" || \
		git clone --depth 1 --branch $(CROSSPLANE_FORK_BRANCH) $(CROSSPLANE_FORK_URL) $(CROSSPLANE_SRC_DIR)
	@git -C $(CROSSPLANE_SRC_DIR) fetch --depth 1 origin $(CROSSPLANE_FORK_BRANCH)
	@git -C $(CROSSPLANE_SRC_DIR) checkout -q FETCH_HEAD
	@$(INFO) pinning buildVersion to $(CROSSPLANE_BUILD_VERSION)
	@sed -i 's|buildVersion = null;|buildVersion = "$(CROSSPLANE_BUILD_VERSION)";|' $(CROSSPLANE_SRC_DIR)/flake.nix
	@grep -q 'buildVersion = "$(CROSSPLANE_BUILD_VERSION)"' $(CROSSPLANE_SRC_DIR)/flake.nix || { \
		echo "could not pin buildVersion in $(CROSSPLANE_SRC_DIR)/flake.nix"; exit 1; }
	@nix build $(CROSSPLANE_SRC_DIR) --option warn-dirty false --print-build-logs
	@$(INFO) build outputs:
	@nix build $(CROSSPLANE_SRC_DIR) --option warn-dirty false --no-link --print-out-paths 2>/dev/null || true
	@$(OK) built. Load the image into kind with: \
		kind load docker-image <image-ref> -n $(KIND_CLUSTER_NAME)

crossplane_sentinel = $(kind_dir)/crossplane_sentinel
# TEST:integration
ENVTEST_ADDITIONAL_FLAGS ?= --bin-dir "$(kind_dir)"
# See https://storage.googleapis.com/kubebuilder-tools/ for list of supported K8s versions
ENVTEST_K8S_VERSION = 1.26.x
INTEGRATION_TEST_DEBUG_OUTPUT ?= false

# MinIO chart/app versions used for end to end tests. Pinned so a chart bump
# cannot silently change what the suite runs against.
MINIO_CHART_VERSION ?= 5.0.7
MINIO_NAMESPACE    ?= minio
MINIO_SERVICE      ?= minio-server
# MinIO's own published images are no longer pullable anonymously:
# quay.io/minio/minio and docker.io/minio/minio both return 401 with a valid
# anonymous token, ghcr.io/minio/minio returns 403, and bitnami/minio has been
# removed, with no public mirror. pgsty/minio is an anonymously pullable mirror
# of the upstream binary and is verified to run. Pin the tag rather than
# floating on latest, so the suite cannot change under us.
MINIO_IMAGE_REPOSITORY ?= pgsty/minio
MINIO_IMAGE_TAG        ?= RELEASE.2026-08-04T00-00-00Z
# The chart's post-install job uses a second, separate image (mcImage) to create a
# default user, and it is also 401 on quay.io. helm --wait blocks on the hook, so
# the suite hangs unless this is redirected too. pgsty/mc is the same mirror.
MINIO_MC_IMAGE_REPOSITORY ?= pgsty/mc
MINIO_MC_IMAGE_TAG        ?= RELEASE.2026-03-21T00-00-00Z
# A cold runner has to pull the MinIO image before the chart becomes ready, so
# this is deliberately generous. The first CI run failed at 5m with a bare
# "context deadline exceeded", which says nothing about the cause.
MINIO_WAIT_TIMEOUT ?= 10m
# The chart requests 2Gi by default, which the kind node cannot satisfy once
# Crossplane and the provider are running, and the pod stays Pending with
# "Insufficient memory". The previous test/minio/values.yaml asked for 128Mi and
# this must be kept explicitly, because the default is far too large.
MINIO_MEMORY_REQUEST ?= 128Mi
MINIO_MEMORY_LIMIT   ?= 512Mi

# kuttl is the test runner. It is deprecated upstream (no release since January
# 2023) and is kept only because test/e2e is already written in its format.
# Migrating to chainsaw, or to uptest >= 1.0 which is built on chainsaw, is the
# real fix; see docs/development.md.
kuttl_bin = $(KUTTL)

mc_bin = $(go_bin)/mc
$(mc_bin): export GOBIN = $(go_bin)
$(mc_bin): | $(go_bin)
	go install github.com/minio/mc@latest

# The build submodule does not create this directory, and the e2e job failed on
# a clean runner with "No rule to make target _output/bin".
$(go_bin):
	@mkdir -p $@

.PHONY: local-install
local-install: kind-load-image crossplane-setup minio-setup package-push-local ## Install Operator in local cluster

# Materialise a kubeconfig for the kind cluster. controlplane.mk switches the
# ambient context, which is not enough: the recipes below must not depend on
# global state, so they are pointed at this file instead.
.PHONY: kind-kubeconfig
kind-kubeconfig: $(KIND)
	@mkdir -p $(kind_dir)
	@$(KIND) get kubeconfig --name $(KIND_CLUSTER_NAME) > $(KIND_KUBECONFIG)
	@chmod 600 $(KIND_KUBECONFIG)
	@$(INFO) wrote $(KIND_KUBECONFIG)

# Chart for the Crossplane control plane, as an OCI reference.
#
# The build submodule's controlplane.up installs Crossplane the classic way:
# `helm repo add <repo>` then `helm install <repo>/crossplane --version <v>`. It
# cannot consume an oci:// reference, and it cannot work here anyway: the
# provider requires Crossplane >= v2.5.0, but the newest chart in
# charts.crossplane.io/stable is 2.4.2, so its `helm install --version 2.5.0`
# fails outright. Serving the chart over a file:// repo is not an option either,
# as helm has no file protocol handler for `helm repo add`.
#
# So the chart is pulled from the OCI registry this project publishes and
# installed directly, and controlplane.up is left out of the dependency graph.
# The chart is the upstream one with image.repository already pointed at
# ghcr.io/rossigee/crossplane, so no image override is needed for the default
# path; the override below is kept for pointing the suite at another build.
#
# The image is also side-loaded into the kind node rather than pulled by the
# node's container runtime. The node fails to pull the multi-arch image from
# ghcr.io with "unable to fetch descriptor ... which reports content size of
# zero", and both Core and rbac-manager then sit in ImagePullBackOff until the
# 10 minute wait expires. A host `docker pull` of the same reference succeeds
# and resolves the index to the amd64 manifest, and the pods become Ready as
# soon as that image is loaded into the node, so the image itself is fine. The
# root cause of the runtime's failure is not established here; loading the
# image avoids it and is the same approach the provider image already uses.
CROSSPLANE_CHART_REFERENCE ?= oci://ghcr.io/rossigee/charts/crossplane

# Bring up the control plane: create the kind cluster if it is not already
# there, materialise its kubeconfig, and install Crossplane from the OCI chart.
# The cluster must exist before kind-kubeconfig can run, so the cluster is
# created here rather than depending on kind-kubeconfig, which fails on a
# cluster that does not exist yet.
.PHONY: crossplane-up
crossplane-up: export KUBECONFIG = $(KIND_KUBECONFIG)
crossplane-up: $(HELM) $(KUBECTL) $(KIND)
	@$(INFO) setting up controlplane
	@$(KIND) get kubeconfig --name $(KIND_CLUSTER_NAME) >/dev/null 2>&1 || $(KIND) create cluster --name=$(KIND_CLUSTER_NAME)
	@mkdir -p $(kind_dir)
	@$(KIND) get kubeconfig --name $(KIND_CLUSTER_NAME) > $(KIND_KUBECONFIG)
	@chmod 600 $(KIND_KUBECONFIG)
	@$(INFO) loading Crossplane image into the kind node
	@command -v $(DOCKER) >/dev/null 2>&1 || { \
		echo "docker is required to side-load the Crossplane image into kind."; \
		echo "The node's container runtime cannot pull the multi-arch image from ghcr,"; \
		echo "so it is pulled with a host runtime and loaded instead."; \
		exit 1; \
	}
	@$(DOCKER) pull $(CROSSPLANE_IMAGE_REPOSITORY):$(CROSSPLANE_IMAGE_TAG)
	@$(KIND) load docker-image $(CROSSPLANE_IMAGE_REPOSITORY):$(CROSSPLANE_IMAGE_TAG) -n $(KIND_CLUSTER_NAME)
	@$(INFO) installing Crossplane $(CROSSPLANE_VERSION) from $(CROSSPLANE_CHART_REFERENCE)
	@$(HELM) upgrade --install crossplane $(CROSSPLANE_CHART_REFERENCE) \
		--version $(CROSSPLANE_VERSION) \
		--create-namespace --namespace crossplane-system \
		$(if $(CROSSPLANE_ARGS),--set "args={$(CROSSPLANE_ARGS)}",) \
		--wait --timeout $(CROSSPLANE_WAIT_TIMEOUT) || { \
		$(INFO) Crossplane did not become ready, dumping state; \
		$(KUBECTL) -n crossplane-system get pods -o wide || true; \
		$(KUBECTL) -n crossplane-system describe pod -l pkg.crossplane.io/provider=crossplane || true; \
		$(KUBECTL) -n crossplane-system get events --sort-by=.lastTimestamp | tail -25 || true; \
		exit 1; \
	}

# Install Crossplane, optionally overriding the controller image so the suite can
# run against a different 2.5.0 build. The default chart already points at
# ghcr.io/rossigee/crossplane, so this is a no-op unless the image variables are
# overridden; it is kept so CROSSPLANE_IMAGE_REPOSITORY still works.
.PHONY: crossplane-setup
crossplane-setup: crossplane-up
	@if [ -n "$(CROSSPLANE_IMAGE_REPOSITORY)" ]; then \
		$(INFO) overriding Crossplane image with $(CROSSPLANE_IMAGE_REPOSITORY):$(CROSSPLANE_IMAGE_TAG); \
		$(KUBECTL) -n crossplane-system set image deployment/crossplane \
			crossplane=$(CROSSPLANE_IMAGE_REPOSITORY):$(CROSSPLANE_IMAGE_TAG); \
		$(KUBECTL) -n crossplane-system rollout status deployment/crossplane --timeout=300s; \
	fi

# MinIO runs in-cluster with no ingress. The provider and the tests both reach it
# over cluster DNS (http://minio-server.minio.svc:9000), which is what makes the
# suite runnable on a CI runner: the previous configuration drove test uploads
# through an ingress at minio.127.0.0.1.nip.io, which needs public DNS and
# ingress-nginx and therefore could never work in CI.
minio-setup: export KUBECONFIG = $(KIND_KUBECONFIG)
minio-setup: $(HELM) kind-kubeconfig
	@$(INFO) installing MinIO $(MINIO_CHART_VERSION)
	@$(HELM) repo add minio https://charts.min.io/ --force-update
	@$(HELM) repo update minio
	@# --no-hooks skips the chart's post-install job. That job runs
	@# /bin/sh /config/add-user, which reads /config/rootUser, but the chart only
	@# mounts a secret there when existingSecret and existingSecretKey are set, and
	@# existingSecretKey is not a value this chart version defines. Without them the
	@# job aborts on `cat: /config/rootUser: No such file or directory` and, because
	@# helm --wait covers hooks, the whole install fails even though the MinIO server
	@# is healthy. The job only creates a convenience user; the suite authenticates
	@# with rootUser/rootPassword, which reach the server through the chart's own
	@# secret, so nothing the tests rely on is lost.
	@$(HELM) upgrade --install --create-namespace --namespace $(MINIO_NAMESPACE) minio minio/minio \
		--version $(MINIO_CHART_VERSION) \
		--set fullnameOverride=$(MINIO_SERVICE) \
		--set mode=standalone \
		--set persistence.enabled=false \
		--set rootUser=minioadmin \
		--set rootPassword=minioadmin \
		--set ingress.enabled=false \
		$(if $(MINIO_IMAGE_TAG),--set image.tag=$(MINIO_IMAGE_TAG),) \
		--set image.repository=$(MINIO_IMAGE_REPOSITORY) \
		--set mcImage.repository=$(MINIO_MC_IMAGE_REPOSITORY) \
		--set mcImage.tag=$(MINIO_MC_IMAGE_TAG) \
		--no-hooks \
		--set resources.requests.memory=$(MINIO_MEMORY_REQUEST) \
		--set resources.requests.cpu=50m \
		--set resources.limits.memory=$(MINIO_MEMORY_LIMIT) \
		--wait --timeout $(MINIO_WAIT_TIMEOUT) || { \
		$(INFO) MinIO did not become ready, dumping state; \
		$(KUBECTL) -n $(MINIO_NAMESPACE) get pods -o wide || true; \
		$(KUBECTL) -n $(MINIO_NAMESPACE) describe pod -l app=$(MINIO_SERVICE) || true; \
		$(KUBECTL) -n $(MINIO_NAMESPACE) get events --sort-by=.lastTimestamp | tail -25 || true; \
		$(HELM) -n $(MINIO_NAMESPACE) status minio || true; \
		exit 1; \
	}
	@$(KUBECTL) -n $(MINIO_NAMESPACE) rollout status deployment/$(MINIO_SERVICE) --timeout=180s
	@$(OK) MinIO is available in-cluster at http://$(MINIO_SERVICE).$(MINIO_NAMESPACE).svc:9000

.PHONY: provider-config
provider-config: export KUBECONFIG = $(KIND_KUBECONFIG)
provider-config: kind-kubeconfig
	@$(INFO) installing the MinIO credentials secret
	@$(KUBECTL) apply -n crossplane-system -f samples/_secret.yaml
	@$(KUBECTL) apply -f test/providerconfig.yaml
	@$(OK) ProviderConfig installed

###
### Integration Tests
###

setup_envtest_bin = $(go_bin)/setup-envtest
envtest_crd_dir ?= $(kind_dir)/crds

# Prepare binary
$(setup_envtest_bin): export GOBIN = $(go_bin)
$(setup_envtest_bin): | $(go_bin)
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: test-integration
test-integration: export ENVTEST_CRD_DIR = $(envtest_crd_dir)
test-integration: $(setup_envtest_bin) .envtest_crds ## Run integration tests against code
	$(setup_envtest_bin) $(ENVTEST_ADDITIONAL_FLAGS) use '$(ENVTEST_K8S_VERSION)!'
	chmod -R +w $(kind_dir)/k8s
	export KUBEBUILDER_ASSETS="$$($(setup_envtest_bin) $(ENVTEST_ADDITIONAL_FLAGS) use -i -p path '$(ENVTEST_K8S_VERSION)!')" && \
	go test -tags=integration ./...

.envtest_crd_dir:
	@mkdir -p $(envtest_crd_dir)
	@cp -r package/crds $(kind_dir)

.envtest_crds: .envtest_crd_dir

.PHONY: .envtest-clean
.envtest-clean:
	rm -f $(setup_envtest_bin)

###
### Local debugging
###

.PHONY: kind-run-operator
kind-run-operator: export KUBECONFIG = $(KIND_KUBECONFIG)
kind-run-operator: kind-setup
	test -n "$${WEBHOOK_TLS_CERT_DIR:-}"
	test -f "$${WEBHOOK_TLS_CERT_DIR}/tls.crt"
	go run . -v 1 operator --webhook-tls-cert-dir "$${WEBHOOK_TLS_CERT_DIR}"

###
### E2E Tests
### with KUTTL (https://kuttl.dev)
###

# test-e2e brings up a real control plane, installs MinIO and this provider
# side-loaded into it, then runs test/e2e.
#
# The provider is deployed with local.xpkg.deploy.provider, which kind-loads the
# locally built image and installs the package with packagePullPolicy: Never.
# That replaces the previous approach, which needed a twuni docker-registry in
# the cluster plus mirror-setup and package-push-local purely so Crossplane
# could pull an image. No in-cluster registry is required any more.
# Port forward MinIO to the host so the suite's own object uploads can reach it
# with the host-side mc client. This replaces an ingress at
# minio.127.0.0.1.nip.io, which needed public DNS and ingress-nginx and could
# therefore never run on a CI runner. MINIO_ENDPOINT is exported into the kuttl
# recipe, and the test steps in test/e2e read it.
MINIO_LOCAL_PORT ?= 19000
test-e2e: export KUBECONFIG = $(KIND_KUBECONFIG)
test-e2e: export MINIO_ENDPOINT = 127.0.0.1:$(MINIO_LOCAL_PORT)
test-e2e: kind-kubeconfig
test-e2e: $(mc_bin) $(KUTTL) crossplane-up xpkg.build minio-setup local.xpkg.deploy.provider.$(PROJECT_NAME) provider-config
	@$(INFO) port forwarding MinIO on $(MINIO_ENDPOINT)
	@$(KUBECTL) -n $(MINIO_NAMESPACE) port-forward service/$(MINIO_SERVICE) $(MINIO_LOCAL_PORT):9000 >/dev/null 2>&1 & \
		echo $$! > $(kind_dir)/port-forward.pid
	@for i in $$(seq 1 30); do \
		$(KUBECTL) -n $(MINIO_NAMESPACE) exec deploy/$(MINIO_SERVICE) -- true >/dev/null 2>&1 && break; \
		sleep 2; \
	done
	@sleep 3
	@$(INFO) running e2e tests
	@$(KUBECTL) wait --for condition=Healthy provider.pkg.crossplane.io/$(PROJECT_NAME) --timeout 120s
	@$(KUBECTL) -n crossplane-system wait --for condition=Ready \
		$$($(KUBECTL) -n crossplane-system get pods -o name -l pkg.crossplane.io/provider=$(PROJECT_NAME)) --timeout 120s
	@rc=0; $(KUTTL) test ./test/e2e --config ./test/e2e/kuttl-test.yaml --suppress-log=Events || rc=$$?; \
		if [ -f $(kind_dir)/port-forward.pid ]; then kill $$(cat $(kind_dir)/port-forward.pid) 2>/dev/null || true; rm -f $(kind_dir)/port-forward.pid; fi; \
		exit $$rc
	@$(OK) e2e tests passed

run-single-e2e: export KUBECONFIG = $(KIND_KUBECONFIG)
run-single-e2e: test-e2e ## Run specific e2e test with `make run-single-e2e test=<name>`
	@echo "re-run a single test with: $(KUTTL) test ./test/e2e --config ./test/e2e/kuttl-test.yaml --test $(test)"

.PHONY: e2e-clean
e2e-clean: export KUBECONFIG = $(KIND_KUBECONFIG)
e2e-clean:
	@if [ -f "$(KIND_KUBECONFIG)" ]; then \
		$(KUBECTL) delete buckets.minio.m.crossplane.io --all --ignore-not-found; \
		$(KUBECTL) delete users.minio.m.crossplane.io --all --ignore-not-found; \
		$(KUBECTL) delete policies.minio.m.crossplane.io --all --ignore-not-found; \
		$(KUBECTL) delete serviceaccounts.minio.m.crossplane.io --all --ignore-not-found; \
	else \
		echo "no kind cluster context active, nothing to clean"; \
	fi

.PHONY: .e2e-test-clean
.e2e-test-clean: controlplane.down
	rm -f $(mc_bin)
