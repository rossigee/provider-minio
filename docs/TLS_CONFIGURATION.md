# TLS Configuration for provider-minio

This document describes how to configure custom TLS settings for the MinIO provider to support secure connections
with custom Certificate Authorities (CAs), self-signed certificates, and mutual TLS authentication.

## Overview

The MinIO provider supports custom TLS configuration through the `tls` field in the `ProviderConfig`
specification. This allows you to:

- Connect to MinIO instances using custom or internal Certificate Authorities
- Use self-signed certificates in testing environments
- Configure mutual TLS (mTLS) authentication
- Skip TLS verification for testing purposes

## Configuration Options

### Basic TLS Configuration with Custom CA

```yaml
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: provider-config-with-ca
spec:
  credentials:
    apiSecretRef:
      name: minio-secret
      namespace: crossplane-system
    source: Secret
  minioURL: https://minio.example.com:9000/
  tls:
    caSecretRef:
      name: golder-ca-configmap
      key: ca.crt
```

### Mutual TLS Authentication

```yaml
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: provider-config-with-mtls
spec:
  credentials:
    apiSecretRef:
      name: minio-secret
      namespace: crossplane-system
    source: Secret
  minioURL: https://minio.example.com:9000/
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

### Skip TLS Verification (Testing Only)

```yaml
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: provider-config-insecure
spec:
  credentials:
    apiSecretRef:
      name: minio-secret
      namespace: crossplane-system
    source: Secret
  minioURL: https://minio.example.com:9000/
  tls:
    insecureSkipVerify: true
```

## Field Reference

### `tls` Field

The `tls` field is an optional object that configures TLS settings for the MinIO connection.

#### `caSecretRef` (optional)

- **Type**: `corev1.SecretKeySelector`
- **Description**: References a Kubernetes Secret or ConfigMap containing the CA certificate in PEM format for verifying the server's certificate.
- **Fields**:
  - `name`: Name of the Secret or ConfigMap
  - `key`: Key within the Secret/ConfigMap containing the CA certificate
- **Example**:
  ```yaml
  caSecretRef:
    name: ca-certificate-secret
    key: ca.crt
  ```

#### `clientCertSecretRef` (optional)

- **Type**: `corev1.SecretKeySelector`
- **Description**: References a Kubernetes Secret containing the client certificate in PEM format for mutual TLS authentication.
- **Fields**:
  - `name`: Name of the Secret containing the client certificate
  - `key`: Key within the Secret containing the client certificate
- **Note**: Must be used together with `clientKeySecretRef`
- **Example**:
  ```yaml
  clientCertSecretRef:
    name: minio-client-cert
    key: tls.crt
  ```

#### `clientKeySecretRef` (optional)

- **Type**: `corev1.SecretKeySelector`
- **Description**: References a Kubernetes Secret containing the client private key in PEM format for mutual TLS authentication.
- **Fields**:
  - `name`: Name of the Secret containing the client private key
  - `key`: Key within the Secret containing the client private key
- **Note**: Must be used together with `clientCertSecretRef`
- **Example**:
  ```yaml
  clientKeySecretRef:
    name: minio-client-cert
    key: tls.key
  ```

#### `insecureSkipVerify` (optional)

- **Type**: `boolean`
- **Description**: Controls whether the client verifies the server's certificate chain and host name.
- **Default**: `false`
- **Warning**: Setting this to `true` should only be used for testing purposes as it disables certificate validation.

## Use Cases

### Internal Certificate Authority

When your MinIO instance uses certificates signed by an internal CA that is not in the system's trust store:

```yaml
spec:
  minioURL: https://internal-minio.company.local:9000/
  tls:
    caSecretRef:
      name: internal-ca-secret
      key: ca.crt
```

### Self-Signed Certificates (Development)

For development environments with self-signed certificates:

```yaml
spec:
  minioURL: https://dev-minio.local:9000/
  tls:
    caSecretRef:
      name: dev-ca-secret
      key: ca.crt
```

### Corporate Security Requirements

For environments requiring mutual TLS authentication:

```yaml
spec:
  minioURL: https://secure-minio.company.local:9000/
  tls:
    caSecretRef:
      name: company-ca-secret
      key: ca.crt
    clientCertSecretRef:
      name: minio-client-cert
      key: tls.crt
    clientKeySecretRef:
      name: minio-client-cert
      key: tls.key
```

## Creating Required Secrets

### CA Certificate Secret

Create a Secret or ConfigMap containing your CA certificate:

```bash
kubectl create secret generic ca-certificate-secret \
  --from-file=ca.crt=/path/to/your/ca-certificate.pem \
  --namespace=crossplane-system
```

Or using a ConfigMap:

```bash
kubectl create configmap golder-ca-configmap \
  --from-file=ca.crt=/path/to/your/ca-certificate.pem \
  --namespace=crossplane-system
```

### Client Certificate Secret (for mTLS)

Create a Secret containing both client certificate and private key:

```bash
kubectl create secret tls minio-client-cert \
  --cert=/path/to/client-certificate.pem \
  --key=/path/to/client-private-key.pem \
  --namespace=crossplane-system
```

This creates a Secret with standard keys:
- `tls.crt`: Client certificate
- `tls.key`: Client private key

## Security Considerations

1. **Secret Management**: All certificates and private keys are stored as Kubernetes Secrets, following Kubernetes security best practices.

2. **Namespace Security**: Secrets are typically stored in the `crossplane-system` namespace (or the same namespace as your credentials secret).

3. **RBAC**: Ensure proper RBAC permissions are configured for the provider to access the referenced Secrets.

4. **Private Keys**: Private keys are securely stored in Kubernetes Secrets and never exposed in ProviderConfig manifests.

5. **Certificate Rotation**: Update the Secret contents when certificates expire. The provider will pick up changes automatically.

6. **insecureSkipVerify**: Only use this option in development or testing environments.

## Migration from Previous Versions

Previous versions that used inline certificate data (`caData`, `clientCertData`, `clientKeyData`) are no longer supported. You must migrate to using secret references:

1. **Create Secrets**: Store your certificates in Kubernetes Secrets as shown above.
2. **Update ProviderConfig**: Replace inline data fields with secret references.
3. **Test Connection**: Verify the provider can connect using the new configuration.

## Troubleshooting

### Certificate Validation Errors

If you encounter certificate validation errors:

1. Verify the Secret exists and contains the correct CA certificate
2. Check that the MinIO server hostname matches the certificate
3. Ensure the certificate is not expired
4. Verify the Secret is in the correct namespace
5. For testing, temporarily use `insecureSkipVerify: true` to isolate the issue

### Mutual TLS Authentication Failures

If mutual TLS authentication fails:

1. Verify both `clientCertSecretRef` and `clientKeySecretRef` are provided
2. Check that the client certificate Secret exists and contains valid data
3. Ensure the client certificate is signed by a CA trusted by the MinIO server
4. Verify the client certificate is not expired
5. Check RBAC permissions for accessing the client certificate Secret

### Secret Access Issues

If the provider cannot access Secrets:

1. Verify the Secret exists in the expected namespace
2. Check RBAC permissions for the provider service account
3. Ensure the Secret contains the expected keys (`ca.crt`, `tls.crt`, `tls.key`)
4. Verify the Secret data is properly base64 encoded (handled automatically by kubectl)

### Connection Issues

If you cannot connect to MinIO:

1. Check that the `minioURL` is correct and accessible
2. Verify the MinIO server is configured to accept TLS connections
3. Test connectivity without TLS first if possible
4. Check network policies and firewall rules
5. Review provider logs for detailed error messages

## Examples

See the `samples/` directory for complete examples of ProviderConfigs with TLS configuration using secret references.
