# TLS Configuration for provider-minio

This document describes how to configure custom TLS settings for the MinIO provider to support secure connections with custom Certificate Authorities (CAs), self-signed certificates, and mutual TLS authentication.

## Overview

The MinIO provider now supports custom TLS configuration through the `tls` field in the `ProviderConfig` specification. This allows you to:

- Connect to MinIO instances using custom or internal Certificate Authorities
- Use self-signed certificates in testing environments
- Configure mutual TLS (mTLS) authentication
- Skip TLS verification for testing purposes

## Configuration Options

### Basic TLS Configuration with Custom CA

#### Option 1: Inline CA Certificate
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
    caData: |
      -----BEGIN CERTIFICATE-----
      MIIDxTCCAq2gAwIBAgIJAKXGz9P2v7s2MA0GCSqGSIb3DQEBCwUAMHkxCzAJBgNV
      # ... your CA certificate content ...
      -----END CERTIFICATE-----
```

#### Option 2: CA Certificate from Secret
```yaml
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: provider-config-with-ca-secret
spec:
  credentials:
    apiSecretRef:
      name: minio-secret
      namespace: crossplane-system
    source: Secret
  minioURL: https://minio.example.com:9000/
  tls:
    caSecretRef:
      name: minio-ca-cert
      key: ca.crt
```

#### Option 3: CA Certificate from ConfigMap
```yaml
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: provider-config-with-ca-configmap
spec:
  credentials:
    apiSecretRef:
      name: minio-secret
      namespace: crossplane-system
    source: Secret
  minioURL: https://minio.example.com:9000/
  tls:
    caConfigMapRef:
      name: ca-certificates
      key: minio-ca.crt
```

### Mutual TLS Authentication

#### Recommended: Using Secret References
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
      name: minio-ca-cert
      key: ca.crt
    clientCertSecretRef:
      name: minio-client-cert
      key: tls.crt
    clientKeySecretRef:
      name: minio-client-cert
      key: tls.key
```

#### Alternative: Inline Configuration (Not Recommended for Private Keys)
```yaml
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: provider-config-with-mtls-inline
spec:
  credentials:
    apiSecretRef:
      name: minio-secret
      namespace: crossplane-system
    source: Secret
  minioURL: https://minio.example.com:9000/
  tls:
    caData: |
      -----BEGIN CERTIFICATE-----
      # ... your CA certificate content ...
      -----END CERTIFICATE-----
    clientCertData: |
      -----BEGIN CERTIFICATE-----
      # ... your client certificate content ...
      -----END CERTIFICATE-----
    clientKeyData: |  # DEPRECATED: Use clientKeySecretRef instead
      -----BEGIN PRIVATE KEY-----
      # ... your client private key content ...
      -----END PRIVATE KEY-----
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

#### `caData` (optional)
- **Type**: `string`
- **Description**: CA certificate data in PEM format for verifying the server's certificate. This is useful for self-signed certificates or private CA certificates.
- **Format**: PEM-encoded certificate
- **Priority**: Used if provided; otherwise falls back to `caSecretRef` or `caConfigMapRef`

#### `caSecretRef` (optional)
- **Type**: `SecretKeySelector`
- **Description**: Reference to a Secret containing the CA certificate data
- **Fields**:
  - `name`: Name of the Secret (must exist in the `crossplane-system` namespace)
  - `key`: Key within the Secret containing the CA certificate (e.g., `ca.crt`)
- **Priority**: Used if `caData` is not provided
- **Note**: The Secret must be in the `crossplane-system` namespace where the provider is installed

#### `caConfigMapRef` (optional)
- **Type**: `ConfigMapKeySelector`
- **Description**: Reference to a ConfigMap containing the CA certificate data
- **Fields**:
  - `name`: Name of the ConfigMap (must exist in the `crossplane-system` namespace)
  - `key`: Key within the ConfigMap containing the CA certificate
- **Priority**: Used if neither `caData` nor `caSecretRef` are provided
- **Note**: The ConfigMap must be in the `crossplane-system` namespace where the provider is installed

#### `clientCertData` (optional)
- **Type**: `string`
- **Description**: Client certificate data in PEM format for mutual TLS authentication.
- **Format**: PEM-encoded certificate
- **Note**: Must be used together with client key (either `clientKeyData` or `clientKeySecretRef`)
- **Priority**: Used if provided; otherwise falls back to `clientCertSecretRef`

#### `clientCertSecretRef` (optional)
- **Type**: `SecretKeySelector`
- **Description**: Reference to a Secret containing the client certificate data
- **Fields**:
  - `name`: Name of the Secret (must exist in the `crossplane-system` namespace)
  - `key`: Key within the Secret containing the client certificate (e.g., `tls.crt`)
- **Priority**: Used if `clientCertData` is not provided
- **Note**: The Secret must be in the `crossplane-system` namespace where the provider is installed

#### `clientKeyData` (optional) - DEPRECATED
- **Type**: `string`
- **Description**: Client private key data in PEM format for mutual TLS authentication.
- **Format**: PEM-encoded private key
- **Note**: Must be used together with client certificate
- **Warning**: DEPRECATED - Use `clientKeySecretRef` instead. Private keys should not be stored in CRDs.

#### `clientKeySecretRef` (optional) - RECOMMENDED
- **Type**: `SecretKeySelector`
- **Description**: Reference to a Secret containing the client private key data
- **Fields**:
  - `name`: Name of the Secret (must exist in the `crossplane-system` namespace)
  - `key`: Key within the Secret containing the client private key (e.g., `tls.key`)
- **Priority**: Used if `clientKeyData` is not provided
- **Note**: The Secret must be in the `crossplane-system` namespace where the provider is installed

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
    caData: |
      -----BEGIN CERTIFICATE-----
      # Internal CA certificate
      -----END CERTIFICATE-----
```

### Self-Signed Certificates (Development)

For development environments with self-signed certificates:

```yaml
spec:
  minioURL: https://dev-minio.local:9000/
  tls:
    caData: |
      -----BEGIN CERTIFICATE-----
      # Self-signed certificate
      -----END CERTIFICATE-----
```

### Corporate Security Requirements

For environments requiring mutual TLS authentication:

```yaml
spec:
  minioURL: https://secure-minio.company.local:9000/
  tls:
    caData: |
      -----BEGIN CERTIFICATE-----
      # Company CA certificate
      -----END CERTIFICATE-----
    clientCertData: |
      -----BEGIN CERTIFICATE-----
      # Client certificate for authentication
      -----END CERTIFICATE-----
    clientKeyData: |
      -----BEGIN PRIVATE KEY-----
      # Client private key
      -----END PRIVATE KEY-----
```

## Security Considerations

1. **Certificate Storage**: Always use Secret references (`caSecretRef`, `clientCertSecretRef`, `clientKeySecretRef`) instead of inline data for production deployments.
2. **Private Keys**:
   - **NEVER** use `clientKeyData` in production - it's deprecated and insecure
   - Always use `clientKeySecretRef` to reference private keys stored in Kubernetes Secrets
   - Never commit private keys to version control
3. **ConfigMap vs Secret**:
   - Use ConfigMaps only for public CA certificates
   - Always use Secrets for client certificates and private keys
4. **Certificate Rotation**: Plan for certificate rotation by updating the referenced Secrets/ConfigMaps
5. **insecureSkipVerify**: Only use this option in development or testing environments
6. **RBAC**: Ensure the Crossplane provider has appropriate RBAC permissions to read the referenced Secrets and ConfigMaps
7. **Namespace**: All referenced Secrets and ConfigMaps must be in the `crossplane-system` namespace where the provider is installed

## Migration from Previous Versions

If you were previously using MinIO without custom TLS configuration, your existing ProviderConfigs will continue to work without changes. The `tls` field is optional and backwards compatible.

To add TLS configuration to an existing ProviderConfig, simply add the `tls` field with your desired configuration.

## Troubleshooting

### Certificate Validation Errors

If you encounter certificate validation errors:

1. Verify the CA certificate is correct and properly formatted
2. Check that the MinIO server hostname matches the certificate
3. Ensure the certificate is not expired
4. For testing, temporarily use `insecureSkipVerify: true` to isolate the issue

### Mutual TLS Authentication Failures

If mutual TLS authentication fails:

1. Verify both `clientCertData` and `clientKeyData` are provided
2. Check that the client certificate is signed by a CA trusted by the MinIO server
3. Ensure the client certificate is not expired
4. Verify the private key matches the client certificate

### Connection Issues

If you cannot connect to MinIO:

1. Check that the `minioURL` is correct and accessible
2. Verify the MinIO server is configured to accept TLS connections
3. Test connectivity without TLS first if possible
4. Check network policies and firewall rules

## Examples

See the `samples/` directory for complete examples of ProviderConfigs with TLS configuration.