# Configuration Guide

This guide covers `ProviderConfig` for `provider-minio` — credentials, endpoint, and TLS.

## ProviderConfig (Cluster-Scoped)

`ProviderConfig` is **cluster-scoped** (`apis/provider/v1/providerconfig_types.go:44`).
Managed resources (`minio.m.crossplane.io/v1beta1` namespaced) reference it via `spec.providerConfigRef.name`.

### Minimal Example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: minio-credentials
  namespace: crossplane-system
type: Opaque
stringData:
  # Keys expected by operator/minioutil/client.go:40
  AWS_ACCESS_KEY_ID: minioadmin
  AWS_SECRET_ACCESS_KEY: minioadmin
---
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: default
spec:
  minioURL: https://minio.example.com:9000
  credentials:
    source: Secret
    apiSecretRef:
      name: minio-credentials
      namespace: crossplane-system
```

Apply:

```bash
kubectl apply -f secret.yaml
kubectl apply -f providerconfig.yaml
kubectl get providerconfig
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.minioURL` | string | yes | MinIO endpoint URL (`https://…` => `Secure: true`, `http://…` => `Secure: false`). Parsed via `net/url.Parse` (`operator/minioutil/client.go:34`). |
| `spec.credentials.source` | enum | yes | `Secret`, `InjectedIdentity`, `Environment`, `Filesystem`, `None` (`apis/provider/v1/providerconfig_types.go:27`). Only `Secret` is fully wired (`internal/clients/minio.go:36`). |
| `spec.credentials.apiSecretRef` | `SecretReference` | when `source: Secret` | Secret containing MinIO keys. The active client (`operator/minioutil/client.go:28`) reads `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`; the legacy `internal/clients/minio.go:70` reads `accessKey` / `secretKey` or JSON via `secretRef.key` (`internal/clients/minio.go:45`). Provide both key styles for compatibility. |
| `spec.credentials.secretRef` | `SecretReference` + `key` | alt | JSON-blob variant (`internal/clients/minio.go:50`): secret `data[key]` is JSON `{"endpoint":…,"accessKey":…,"secretKey":…}`. Rarely needed. |
| `spec.tls` | `TLSConfig` | no | See `apis/common/common.go:23` and `docs/TLS_CONFIGURATION.md`. |

### Credentials Secret

Canonical secret created by `generate_sample.go:103`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: minio-secret
  namespace: crossplane-system
data:
  AWS_ACCESS_KEY_ID: bWluaW9hZG1pbg==  # minioadmin
  AWS_SECRET_ACCESS_KEY: bWluaW9hZG1pbg==
```

If you use `internal/clients` path, keys are `accessKey`/`secretKey` (`internal/clients/minio.go:70`). To be safe, store both:

```bash
kubectl create secret generic minio-credentials \
  --from-literal=AWS_ACCESS_KEY_ID=minioadmin \
  --from-literal=AWS_SECRET_ACCESS_KEY=minioadmin \
  --from-literal=accessKey=minioadmin \
  --from-literal=secretKey=minioadmin \
  -n crossplane-system
```

### TLS

Use `spec.tls` to supply custom CA, mTLS client cert/key, or skip verification (testing only). See `docs/TLS_CONFIGURATION.md` for full examples.

```yaml
spec:
  minioURL: https://minio.example.com:9000
  tls:
    caSecretRef:
      name: ca-cert
      key: ca.crt
    # caConfigMapRef:
    #   name: ca-bundle
    #   key: ca.crt
    # clientCertSecretRef:
    #   name: minio-client-cert
    #   key: tls.crt
    # clientKeySecretRef:
    #   name: minio-client-cert
    #   key: tls.key
    # insecureSkipVerify: false
```

TLS data resolution order per key (`operator/minioutil/client.go:117`): `inlineData` (`caData`, `clientCertData`, `clientKeyData`) > `SecretRef` > `ConfigMapRef` (CA only). Secrets/ConfigMaps are looked up in the same namespace as `apiSecretRef` (`operator/minioutil/client.go:47`).

### Multiple ProviderConfigs

```yaml
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: prod
spec:
  minioURL: https://prod-minio.example.com:9000
  credentials:
    source: Secret
    apiSecretRef: { name: prod-creds, namespace: crossplane-system }
---
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: dev
spec:
  minioURL: http://dev-minio.minio.svc:9000
  credentials:
    source: Secret
    apiSecretRef: { name: dev-creds, namespace: crossplane-system }
```

Reference from managed resources:

```yaml
spec:
  providerConfigRef:
    name: prod
```

If omitted, defaults to `default` (`package/crds/minio.m.crossplane.io_buckets.yaml:135`).

## Troubleshooting

* `cannot get provider config` — ProviderConfig name mismatch or not created.
* `cannot get connection secret` / `no secret reference provided` — `apiSecretRef` missing or wrong namespace/key (`internal/clients/minio.go:52`).
* TLS `failed to parse CA certificate` — ensure PEM `-----BEGIN CERTIFICATE-----` in `ca.crt`.
* `both client certificate and key must be provided for mutual TLS` (`operator/minioutil/client.go:109`).

## See Also

* `docs/TLS_CONFIGURATION.md` — TLS examples and secret creation
* `docs/API.md` — managed resource specs
* `docs/ServiceAccount.md` — service accounts
* `samples/` — generated examples (legacy v1-style, see `examples/v2/` for v1beta1)
