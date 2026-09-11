# TLS Configuration for provider-minio

This document describes custom TLS settings for the MinIO provider (`spec.tls` in `ProviderConfig` `minio.crossplane.io/v1`).

> **Types:** `apis/common/common.go:23` `TLSConfig`, wired in `operator/minioutil/client.go:44`.
> **ProviderConfig:** `apis/provider/v1/providerconfig_types.go:22` cluster-scoped.
> See `docs/CONFIGURATION.md` for ProviderConfig basics and `docs/API.md` for other resources.

## Overview

`spec.tls` allows you to:

* Connect via custom/internal CA
* Use self-signed certificates
* Configure mutual TLS (mTLS)
* Skip verification for testing (`insecureSkipVerify`)

## Configuration Options

### 1. Custom CA via Secret (Recommended)

```yaml
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: provider-config-with-ca
spec:
  minioURL: https://minio.example.com:9000
  credentials:
    source: Secret
    apiSecretRef:
      name: minio-credentials
      namespace: crossplane-system
  tls:
    caSecretRef:
      name: ca-certificate-secret
      key: ca.crt
```

### 2. Inline CA Certificate Data

```yaml
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: provider-config-with-ca-data
spec:
  credentials:
    source: Secret
    apiSecretRef:
      name: minio-credentials
      namespace: crossplane-system
  minioURL: https://minio.example.com:9000
  tls:
    caData: |
      -----BEGIN CERTIFICATE-----
      MIIDxTCCAq2gAwIBAgIJAKXGz9P2v7s2MA0GCSqGSIb3DQEBCwUAMHkxCzAJBgNV
      # ... your CA certificate content ...
      -----END CERTIFICATE-----
```

> Inline `caData` is supported (`apis/common/common.go:27`) but prefer `caSecretRef`/`caConfigMapRef` for rotation.

### 3. CA from ConfigMap

Useful when CA is managed by cert-manager or shared across apps.

```yaml
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: provider-config-with-ca-configmap
spec:
  credentials:
    source: Secret
    apiSecretRef:
      name: minio-credentials
      namespace: crossplane-system
  minioURL: https://minio.example.com:9000
  tls:
    caConfigMapRef:
      name: ca-certificates
      key: minio-ca.crt
```

### 4. Mutual TLS (mTLS)

```yaml
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: provider-config-with-mtls
spec:
  credentials:
    source: Secret
    apiSecretRef:
      name: minio-credentials
      namespace: crossplane-system
  minioURL: https://minio.example.com:9000
  tls:
    caSecretRef:
      name: ca-certificate-secret
      key: ca.crt
    clientCertSecretRef:
      name: minio-client-cert
      key: tls.crt
    clientKeySecretRef:
      name: minio-client-cert
      key: tls.key
```

Inline variant for client cert/key also supported (`clientCertData`, `clientKeyData` in `apis/common/common.go:41,51`).

### 5. Skip TLS Verification (Testing Only)

```yaml
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: provider-config-insecure
spec:
  credentials:
    source: Secret
    apiSecretRef:
      name: minio-credentials
      namespace: crossplane-system
  minioURL: https://minio.example.com:9000
  tls:
    insecureSkipVerify: true
```

## Field Reference

`apis/common/common.go:23`

### `tls` (optional)

#### `caData` (optional, string)

PEM CA bundle inline. Prefer `caSecretRef`/`caConfigMapRef`.

#### `caSecretRef` (optional, `SecretKeySelector`)

* `name` / `key` (key commonly `ca.crt` or `tls.crt`) — `apis/common/common.go:32`

#### `caConfigMapRef` (optional, `ConfigMapKeySelector`)

* `name` / `key` — `apis/common/common.go:37`

#### `clientCertData` (optional, string)

Inline client certificate PEM (`apis/common/common.go:41`).

#### `clientCertSecretRef` (optional, `SecretKeySelector`)

* Must be paired with `clientKeySecretRef` (`operator/minioutil/client.go:109`)

#### `clientKeyData` (optional, string, deprecated)

Inline private key PEM. Prefer `clientKeySecretRef` (comment in `common.go:49`).

#### `clientKeySecretRef` (optional, `SecretKeySelector`)

#### `insecureSkipVerify` (optional, bool, default `false`)

Disables chain/host verification (`common.go:62`). Testing only. Annotated `#nosec G402` in `operator/minioutil/client.go:74`.

Resolution order per `operator/minioutil/client.go:117`: inline data > `SecretRef` > `ConfigMapRef` (CA only). Lookup namespace is `spec.credentials.apiSecretRef.namespace` (`client.go:47`).

## Use Cases

### Internal CA

```yaml
spec:
  minioURL: https://internal-minio.company.local:9000
  tls:
    caSecretRef: { name: internal-ca-secret, key: ca.crt }
```

### Self-Signed (Development)

```yaml
spec:
  minioURL: https://dev-minio.local:9000
  tls:
    caSecretRef: { name: dev-ca-secret, key: ca.crt }
```

### Corporate mTLS

```yaml
spec:
  minioURL: https://secure-minio.company.local:9000
  tls:
    caSecretRef: { name: company-ca-secret, key: ca.crt }
    clientCertSecretRef: { name: minio-client-cert, key: tls.crt }
    clientKeySecretRef: { name: minio-client-cert, key: tls.key }
```

## Creating Required Secrets

### CA Secret / ConfigMap

```bash
kubectl create secret generic ca-certificate-secret \
  --from-file=ca.crt=/path/to/ca.pem \
  -n crossplane-system

# Or ConfigMap
kubectl create configmap ca-certificates \
  --from-file=minio-ca.crt=/path/to/ca.pem \
  -n crossplane-system
```

### Client Certificate Secret (mTLS)

```bash
kubectl create secret tls minio-client-cert \
  --cert=/path/to/client.pem \
  --key=/path/to/client-key.pem \
  -n crossplane-system
# Creates keys tls.crt / tls.key
```

## Security Considerations

1. Store certs/keys in Secrets in `crossplane-system` (or same namespace as `apiSecretRef`).
2. RBAC: provider ServiceAccount must be able to `get` Secrets/ConfigMaps in that namespace.
3. Private keys should use `clientKeySecretRef`, not inline `clientKeyData` (deprecated).
4. Rotate Secrets in place; provider picks up changes on next reconcile (TLS config is read at client creation `operator/minioutil/client.go:47`).
5. `insecureSkipVerify: true` only for development/testing.

## Troubleshooting

### Certificate Validation Errors

1. Secret exists and contains correct PEM
2. Hostname matches certificate SAN
3. Certificate not expired
4. Secret in correct namespace (same as `apiSecretRef.namespace`)
5. Temporarily set `insecureSkipVerify: true` to isolate

### mTLS Failures

1. Both `clientCertSecretRef` and `clientKeySecretRef` set
2. Secret exists, keys `tls.crt`/`tls.key` present
3. Client cert signed by CA trusted by MinIO server
4. Client cert not expired
5. RBAC allows provider to `get` secret

### Secret Access Issues

1. Secret exists in expected namespace
2. RBAC for provider ServiceAccount
3. Key names match (`ca.crt`, `tls.crt`, `tls.key`)
4. Data is base64-encoded correctly (kubectl handles this)

### Connection Issues

1. `minioURL` correct and reachable
2. MinIO configured for TLS if `https://` used
3. Test without TLS first (`http://`)
4. Check network policies / provider logs

## Examples

* `samples/providerconfig-tls-configmap.yaml` — ConfigMap CA example
* `samples/tls-resources.yaml` — Secrets/ConfigMaps for TLS
* `samples/minio.crossplane.io_providerconfig_with_tls.yaml`
* `docs/CONFIGURATION.md`
