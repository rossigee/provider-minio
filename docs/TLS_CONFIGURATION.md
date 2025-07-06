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
    caData: |
      -----BEGIN CERTIFICATE-----
      # ... your CA certificate content ...
      -----END CERTIFICATE-----
    clientCertData: |
      -----BEGIN CERTIFICATE-----
      # ... your client certificate content ...
      -----END CERTIFICATE-----
    clientKeyData: |
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

#### `clientCertData` (optional)
- **Type**: `string`
- **Description**: Client certificate data in PEM format for mutual TLS authentication.
- **Format**: PEM-encoded certificate
- **Note**: Must be used together with `clientKeyData`

#### `clientKeyData` (optional)
- **Type**: `string`
- **Description**: Client private key data in PEM format for mutual TLS authentication.
- **Format**: PEM-encoded private key
- **Note**: Must be used together with `clientCertData`

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

1. **Certificate Storage**: Store certificates as Kubernetes secrets and reference them in your ProviderConfig when possible.
2. **Private Keys**: Never commit private keys to version control. Use secret management systems.
3. **Certificate Rotation**: Plan for certificate rotation by updating the ProviderConfig when certificates expire.
4. **insecureSkipVerify**: Only use this option in development or testing environments.

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