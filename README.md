# provider-minio

[![CI](https://img.shields.io/github/actions/workflow/status/rossigee/provider-minio/ci.yml?branch=master)][build]
![Go version](https://img.shields.io/github/go-mod/go-version/rossigee/provider-minio)
[![Version](https://img.shields.io/github/v/release/rossigee/provider-minio)][releases]
[![GitHub downloads](https://img.shields.io/github/downloads/rossigee/provider-minio/total)][releases]

[build]: https://github.com/rossigee/provider-minio/actions/workflows/ci.yml
[releases]: https://github.com/rossigee/provider-minio/releases

**✅ BUILD STATUS: WORKING** - Successfully builds and passes all tests (v0.17.2)

Crossplane provider for managing MinIO object storage resources including buckets, users, and policies.

## ⚡ **BREAKING CHANGE: v1→v2 API Migration**

**Starting with v0.17.0, this provider uses Crossplane v2 namespaced APIs:**

- **Old**: `minio.crossplane.io/v1` (cluster-scoped)
- **New**: `minio.m.crossplane.io/v1beta1` (namespaced) ✅

**See [API Migration Guide](#api-migration-v1v2) below for upgrade instructions.**

## Features

- **Bucket Management**: Create, configure, and manage MinIO buckets
- **User Management**: User accounts and access management
- **Policy Management**: Fine-grained access control policies
- **Service Account Management**: MinIO service accounts for programmatic access
- **TLS Support**: Custom certificate configuration for secure connections
- **Crossplane v2**: Full support for namespaced resources and multi-tenancy
- **Provider Status**: ✅ Production ready with standardized CI/CD pipeline

## Container Registry

- **Primary**: `ghcr.io/rossigee/provider-minio:v0.17.2` ✅
- **Harbor**: Available via environment configuration
- **Upbound**: Available via environment configuration

## API Migration (v1→v2)

### Quick Migration Example

**❌ Old v1 (cluster-scoped):**

```yaml
apiVersion: minio.crossplane.io/v1
kind: Bucket
metadata:
  name: my-bucket  # No namespace
```

**✅ New v1beta1 (namespaced):**

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: Bucket
metadata:
  name: my-bucket
  namespace: production  # Namespace required
```

### Migration Steps

1. **Update API Version**: `minio.crossplane.io/v1` → `minio.m.crossplane.io/v1beta1`
2. **Add Namespace**: All resources now require a namespace
3. **Update Provider Package**: Use v0.16.5+ for v2 support
4. **Test Resources**: Verify resources work in target namespaces

### Compatibility

| Version | v1 APIs | v1beta1 APIs | Crossplane |
|---------|---------|--------------|-------------|
| < v0.16.5 | ✅ | ❌ | v1.x |
| v0.16.5+ | ❌ | ✅ | v2.x |

**Note**: This is a **breaking change**. v1 and v1beta1 APIs cannot coexist.

Documentation: <https://vshn.github.io/provider-minio/provider-minio/>

## Local Development

### Requirements

- `docker`
- `go`
- `helm`
- `kubectl`
- `yq`
- `sed` (or `gsed` for Mac)
- `pre-commit` (optional, for development with quality gates)

Some other requirements (e.g. `kind`) will be compiled on-the-fly and put in the local cache dir `.kind` as needed.

### Common make targets

- `make build` to build the binary and docker image
- `make generate` to (re)generate additional code artifacts
- `make lint` to run linting and code quality checks
- `make test` run test suite
- `make local-install` to install the operator in local cluster
- `make install-samples` to run the provider in local cluster and apply sample manifests
- `make run-operator` to run the code in operator mode against your current kubecontext

See all targets with `make help`

### Development Quality Gates

This project uses comprehensive quality gates to ensure code quality:

- **Pre-commit hooks**: Automatically run linting, formatting, and security checks
- **Go linting**: golangci-lint with strict rules
- **YAML/Markdown linting**: yamllint and markdownlint
- **Security scanning**: Private key detection and secret validation
- **Code formatting**: Automatic go fmt, go imports

To set up pre-commit hooks:

```bash
pip install pre-commit
pre-commit install
```

### QuickStart Demonstration

1. Make sure you have a kind cluster running and the config exported
2. `make local-install`

### Kubernetes Webhook Troubleshooting

The provider comes with mutating and validation admission webhook server.

To test and troubleshoot the webhooks on the cluster, simply apply your changes with `kubectl`.

1. Make sure you have all CRDs and validation webhook registrations installed.

   ```bash
   make install-crd
   kubectl apply -f package/webhook
   ```

2. To debug the webhook in an IDE, we need to generate certificates:

   ```bash
   make webhook-debug
   # if necessary with another endpoint name, depending on your docker setup
   # if you change the webhook_service_name variable, you need to clean out the old certificates
   make webhook-debug -e webhook_service_name=$HOSTIP
   ```

3. Start the operator in your IDE with `WEBHOOK_TLS_CERT_DIR` environment set to `.work/webhooks`.

4. Apply the samples to test the webhooks:

   ```bash
   make install-samples
   ```

### Run operator in debugger

- `make crossplane-setup minio-setup install-crds` to install crossplane and minio in the kind cluster
- `kubectl apply -f samples/_secret.yaml samples/minio.crossplane.io_providerconfig.yaml`
- `export KUBECONFIG=.work/kind/kind-kubeconfig`
- `go run ./cmd/provider --debug`

### Crossplane Provider Mechanics

For detailed information on how Crossplane Provider works from a development perspective check
[provider mechanics documentation page](https://kb.vshn.ch/app-catalog/explanations/crossplane_provider_mechanics.html).

### e2e testing with kuttl

Some scenarios are tested with the Kubernetes E2E testing tool [Kuttl](https://kuttl.dev/docs).
Kuttl is basically comparing the installed manifests (usually files named `##-install*.yaml`) with observed
objects and compares the desired output (files named `##-assert*.yaml`).

To execute tests, run `make test-e2e` from the root dir.

If a test fails, kuttl leaves the resources in the kind-cluster intact, so you can inspect the resources and events if necessary.
Please note that Kubernetes Events from cluster-scoped resources appear in the `default` namespace only,
but `kubectl describe ...` should show you the events.

### Cleaning up e2e tests

`make clean`
