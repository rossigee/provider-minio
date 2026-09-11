# Installation Guide

## Prerequisites

* Kubernetes cluster with [Crossplane](https://docs.crossplane.io) >= `v2.0.0` (`crossplane.yaml:64`, `Makefile:74` `CROSSPLANE_VERSION = 2.4.0`)
* `kubectl`, `helm`, `yq` (see `README.md:163` Requirements)
* MinIO deployment reachable from the cluster

## 1. Install Crossplane

```bash
helm repo add crossplane https://charts.crossplane.io/stable
helm repo update
helm upgrade --install crossplane crossplane/crossplane \
  --namespace crossplane-system --create-namespace --wait
```

Verify:

```bash
kubectl get pods -n crossplane-system
```

## 2. Install Provider

Choose a released version (`VERSION` file is `v0.19.9`; `README.md` example may lag):

```bash
# Using Crossplane CLI (v2)
kubectl crossplane install provider ghcr.io/rossigee/provider-minio:v0.19.9

# Or via Provider manifest
kubectl apply -f - <<EOF
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-minio
spec:
  package: ghcr.io/rossigee/provider-minio:v0.19.9
EOF
```

Wait until healthy:

```bash
kubectl get providers -w
kubectl get providerrevisions
```

## 3. Create Credentials Secret

```bash
kubectl create secret generic minio-credentials \
  --from-literal=AWS_ACCESS_KEY_ID=minioadmin \
  --from-literal=AWS_SECRET_ACCESS_KEY=minioadmin \
  --from-literal=accessKey=minioadmin \
  --from-literal=secretKey=minioadmin \
  -n crossplane-system
```

The provider reads `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` (`operator/minioutil/client.go:40`); the legacy client reads `accessKey`/`secretKey` (`internal/clients/minio.go:70`). Providing both avoids surprises.

## 4. Create ProviderConfig (Cluster-Scoped)

```yaml
# providerconfig.yaml
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: default
spec:
  minioURL: http://minio.minio.svc:9000
  credentials:
    source: Secret
    apiSecretRef:
      name: minio-credentials
      namespace: crossplane-system
      # key is unused for apiSecretRef path; data keys are AWS_ACCESS_KEY_ID etc.
```

```bash
kubectl apply -f providerconfig.yaml
kubectl get providerconfig
```

See `docs/CONFIGURATION.md` for TLS, multiple configs, and troubleshooting.

## 5. Create a Managed Resource (Namespaced)

Managed resources are **namespaced** (`minio.m.crossplane.io/v1beta1`):

```yaml
# bucket.yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: Bucket
metadata:
  name: my-bucket
  namespace: default
spec:
  forProvider:
    region: us-east-1
  providerConfigRef:
    name: default
```

```bash
kubectl apply -f bucket.yaml
kubectl get buckets -A
kubectl wait --for=condition=Ready bucket/my-bucket -n default --timeout=60s
```

## Next Steps

* `docs/API.md` — full resource reference (Bucket, Policy, User, ServiceAccount, NotificationConfiguration)
* `docs/CONFIGURATION.md` — credentials and TLS
* `docs/ServiceAccount.md` — programmatic access
* `docs/TLS_CONFIGURATION.md` — custom CA / mTLS
* `examples/v2/` — hand-written v1beta1 examples
* `samples/` — generated samples (legacy style)

## Local Development

```bash
make submodules
make build          # binary + image
make generate       # code gen
make lint
make local-install  # install provider in current kube context
make install-samples
```

See `README.md:159` and `docs/DEVELOPMENT.md`.

## Upgrading from v1 (Cluster-Scoped) to v2 (Namespaced)

See `README.md:98` Breaking Change and `docs/API.md`. Key steps:

1. Update `apiVersion` from `minio.crossplane.io/v1` → `minio.m.crossplane.io/v1beta1`
2. Add `metadata.namespace` to every managed resource (they are now `scope: Namespaced` per `package/crds/minio.m.crossplane.io_buckets.yaml:18`)
3. Keep `ProviderConfig` as `minio.crossplane.io/v1` cluster-scoped (unchanged)
4. Use provider `v0.16.5+` with Crossplane `v2.x`
