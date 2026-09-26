# Scratch directory for test state. The build submodule does not define
# kind_dir or go_bin, which is part of why this file had no working includer.
kind_dir ?= $(OUTPUT_DIR)/test
go_bin    ?= $(OUTPUT_DIR)/bin

# The e2e suite gets its own kubeconfig rather than relying on whatever context
# the ambient KUBECONFIG happens to point at. A shared kubeconfig means a stray
# current-context can make `make test-e2e` install a provider into a real
# cluster.
KIND_KUBECONFIG ?= $(kind_dir)/kubeconfig

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

# kuttl is the test runner. It is deprecated upstream (no release since January
# 2023) and is kept only because test/e2e is already written in its format.
# Migrating to chainsaw, or to uptest >= 1.0 which is built on chainsaw, is the
# real fix; see docs/development.md.
kuttl_bin = $(KUTTL)

mc_bin = $(go_bin)/mc
$(mc_bin): export GOBIN = $(go_bin)
$(mc_bin): | $(go_bin)
	go install github.com/minio/mc@latest

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

.PHONY: crossplane-setup
crossplane-setup: controlplane.up ## Installs Crossplane in the kind cluster.

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
	@$(HELM) upgrade --install --create-namespace --namespace $(MINIO_NAMESPACE) minio minio/minio \
		--version $(MINIO_CHART_VERSION) \
		--set fullnameOverride=$(MINIO_SERVICE) \
		--set mode=standalone \
		--set persistence.enabled=false \
		--set rootUser=minioadmin \
		--set rootPassword=minioadmin \
		--set ingress.enabled=false \
		--wait --timeout 5m
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
test-e2e: $(mc_bin) controlplane.up xpkg.build minio-setup local.xpkg.deploy.provider.$(PROJECT_NAME) provider-config
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
