# Development Guide

## Requirements

* `docker`, `go` (`GO_REQUIRED_VERSION ?= 1.26.6` in `Makefile:12`), `helm`, `kubectl`, `yq`, `sed`/`gsed`
* `pre-commit` (optional) — see `README.md:185`

Kind and other tools are compiled on demand into `.kind`/`.work/`.

## Quick Start

```bash
git clone https://github.com/rossigee/provider-minio && cd provider-minio
make submodules
make build          # binary + image
make generate       # CRDs + deepcopy + samples (generate_sample.go:10)
make lint           # golangci-lint 2.12.2
make test           # go test (parallel NPROCS/2)
```

All make targets: `make help`.

## Common Targets

| Target | Description |
|--------|-------------|
| `make build` | Build binary (`_output/bin/.../provider`) and image |
| `make generate` | Re-generate code artifacts |
| `make lint` | golangci-lint |
| `make test` | `go test` |
| `make xpkg.build` | Build xpkg (`_output/xpkg/.../*.xpkg`) |
| `make local-install` | Install provider in current kube context |
| `make install-samples` | Apply `samples/_secret.yaml` + `samples/minio.crossplane.io_*.yaml` |
| `make install-crds` / `make uninstall-crds` | Apply/delete `package/crds/` |
| `make run` | Run provider locally out-of-cluster (`go run ./cmd/provider --debug`) |

## Code Generation

`make generate` runs:

* `go generate` (see `generate_sample.go:3` — cleans and re-creates `samples/`)
* kubebuilder `controller-gen` for CRDs/deepcopy

If you edit `apis/` types, run `make generate` and commit diffs. CI enforces `make reviewable && git diff --exit-code` (`.github/workflows/ci.yml:90`).

## ProviderConfig vs Managed Resources

* `ProviderConfig` is cluster-scoped `minio.crossplane.io/v1` (`apis/provider/v1/providerconfig_types.go:44`) with `spec.minioURL` + `spec.credentials.apiSecretRef` + `spec.tls`.
* Managed resources are namespaced `minio.m.crossplane.io/v1beta1` (`apis/minio/v1beta1/*.go`) with `spec.providerConfigRef` defaulting to `default`.

See `docs/API.md` and `docs/CONFIGURATION.md`.

## Local Kind Cluster

```bash
make local-install  # or: make crossplane-setup minio-setup install-crds
kubectl apply -f samples/_secret.yaml
kubectl apply -f samples/minio.crossplane.io_providerconfig.yaml
export KUBECONFIG=.work/kind/kind-kubeconfig
go run ./cmd/provider --debug
```

## Webhooks

Provider has validation webhooks per `package/webhook/manifests.yaml` (`/validate-minio-m-crossplane-io-v1beta1-*`).

```bash
make install-crd
kubectl apply -f package/webhook
# debug out-of-cluster
make webhook-debug
WEBHOOK_TLS_CERT_DIR=.work/webhooks go run ./cmd/provider --debug
make install-samples
```

Provider main registers RBAC self-managed (`cmd/provider/main.go:dc3295e`).

## Tests

### Unit

```bash
make test
go test ./... -count=1
```

### E2E (kuttl)

Scenarios in `test/e2e/` (`bucket`, `policy`, `user`, `serviceaccount`) — each has `00-install*.yaml` / `00-assert.yaml`.

```bash
make test-e2e
```

On failure, kuttl leaves resources intact for inspection (`README.md:256`). Note: events for cluster-scoped `ProviderConfig` appear in `default` namespace.

Cleanup: `make clean`.

## Quality Gates

* Pre-commit: `pre-commit install` (golangci-lint, yamllint, markdownlint, secret scan, go fmt/imports).
* CI `.github/workflows/ci.yml:36` — `detect-noop`, `lint`, `check-diff`, `unit-tests`, `security-scan` (govulncheck + gosec), `build-validation`.
* Release `.github/workflows/release.yml` — publish on tag.

## Release Process

Push a SemVer tag with `v` prefix (`v0.19.9`):

```bash
git tag v0.19.10
git push origin v0.19.10
```

Changelog is auto-generated from PR labels (`area:operator` + `bug|enhancement|documentation|change|breaking|dependency`) — legacy note from Antora docs preserved in git history; see `docs/RELEASE.md`.

## Samples vs Examples

* `samples/` — generated via `go generate` (`generate_sample.go:3` `rm -rf ./samples/*`), legacy style.
* `examples/` — hand-written (`examples/v2/bucket-namespaced.yaml`, `examples/v2/user-namespaced.yaml`, `examples/minio.crossplane.io_serviceaccount.yaml`).
* `docs/modules/ROOT/examples/` — removed (was Antora copy of `samples/`).

## Troubleshooting

* `go.cachedir` — build submodule overrides `XDG_CACHE_HOME` to `.work/helm` (`Makefile:89` note).
* `build.init: $(UP) $(CROSSPLANE_CLI)` ensures `up` and `crossplane-cli` in tool cache before builds.
