# MinIO ServiceAccount Resource

The ServiceAccount resource provides declarative management of MinIO
ServiceAccounts through Crossplane. ServiceAccounts enable programmatic access
to MinIO with specific permissions and time-bound credentials.

## Overview

ServiceAccounts in MinIO are specialized credentials that:

- Belong to a parent user
- Can have their own IAM policies
- Support expiration dates for enhanced security
- Generate access/secret key pairs for authentication
- Enable fine-grained permission control

## Resource Specification

### Basic ServiceAccount

```yaml
apiVersion: minio.crossplane.io/v1
kind: ServiceAccount
metadata:
  name: my-app-serviceaccount
spec:
  providerConfigRef:
    name: minio-provider-config
  forProvider:
    name: "Application Service Account"
    description: "Service account for my application"
  writeConnectionSecretsToRef:
    name: my-app-credentials
    namespace: default
```

### ServiceAccount with Custom Policy

```yaml
apiVersion: minio.crossplane.io/v1
kind: ServiceAccount
metadata:
  name: restricted-serviceaccount
spec:
  providerConfigRef:
    name: minio-provider-config
  forProvider:
    name: "Restricted Access Service Account"
    description: "Limited permissions for specific bucket access"
    policy: |
      {
        "Version": "2012-10-17",
        "Statement": [
          {
            "Effect": "Allow",
            "Action": [
              "s3:GetObject",
              "s3:PutObject",
              "s3:ListBucket"
            ],
            "Resource": [
              "arn:aws:s3:::my-app-bucket",
              "arn:aws:s3:::my-app-bucket/*"
            ]
          }
        ]
      }
  writeConnectionSecretsToRef:
    name: restricted-credentials
    namespace: default
```

### ServiceAccount with Expiration

```yaml
apiVersion: minio.crossplane.io/v1
kind: ServiceAccount
metadata:
  name: temporary-serviceaccount
spec:
  providerConfigRef:
    name: minio-provider-config
  forProvider:
    name: "Temporary Service Account"
    description: "Time-limited access for maintenance tasks"
    expiration: "2025-12-31T23:59:59Z"
    policy: |
      {
        "Version": "2012-10-17",
        "Statement": [
          {
            "Effect": "Allow",
            "Action": [
              "s3:*"
            ],
            "Resource": [
              "arn:aws:s3:::maintenance-bucket",
              "arn:aws:s3:::maintenance-bucket/*"
            ]
          }
        ]
      }
  writeConnectionSecretsToRef:
    name: temp-credentials
    namespace: default
```

### ServiceAccount with Custom Credentials

```yaml
apiVersion: minio.crossplane.io/v1
kind: ServiceAccount
metadata:
  name: custom-serviceaccount
spec:
  providerConfigRef:
    name: minio-provider-config
  forProvider:
    name: "Custom Credentials Service Account"
    description: "Service account with predefined access keys"
    accessKey: "CUSTOM_ACCESS_KEY"
    secretKey: "custom-secret-key-minimum-8-chars"
    targetUser: "specific-parent-user"
  writeConnectionSecretsToRef:
    name: custom-credentials
    namespace: default
```

## Field Reference

### ServiceAccountParameters

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | No | Human-readable name for the service account |
| `description` | string | No | Description of the service account's purpose |
| `accessKey` | string | No | Custom access key (3-128 characters). If not specified, MinIO generates one |
| `secretKey` | string | No | Custom secret key (minimum 8 characters). If not specified, MinIO generates one |
| `targetUser` | string | No | Parent user for the service account. Defaults to provider config user |
| `policy` | string | No | JSON IAM policy document. If not specified, inherits parent user policies |
| `expiration` | string | No | ISO 8601 timestamp when the service account expires |

### ServiceAccountProviderStatus

| Field | Type | Description |
|-------|------|-------------|
| `accessKey` | string | The actual access key ID created in MinIO |
| `accountStatus` | string | Status of the service account (enabled/disabled) |
| `parentUser` | string | The user that owns this service account |
| `impliedPolicy` | boolean | Whether the policy is inherited from the parent user |
| `policy` | string | The actual policy document applied to the service account |
| `expiration` | string | When the service account expires (if set) |

## Connection Secrets

ServiceAccounts automatically publish connection credentials to Kubernetes secrets:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-app-credentials
  namespace: default
type: Opaque
data:
  AWS_ACCESS_KEY_ID: <base64-encoded-access-key>
  AWS_SECRET_ACCESS_KEY: <base64-encoded-secret-key>
```

These credentials can be consumed by applications:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      containers:
      - name: app
        image: my-app:latest
        env:
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: my-app-credentials
              key: AWS_ACCESS_KEY_ID
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: my-app-credentials
              key: AWS_SECRET_ACCESS_KEY
        - name: AWS_ENDPOINT_URL
          value: "https://minio.example.com"
```

## Security Best Practices

### 1. Use Least Privilege Policies

Always specify custom policies that grant only the minimum required permissions:

```yaml
policy: |
  {
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Action": [
          "s3:GetObject"
        ],
        "Resource": [
          "arn:aws:s3:::specific-bucket/specific-prefix/*"
        ]
      }
    ]
  }
```

### 2. Set Expiration Dates

For temporary access, always set expiration dates:

```yaml
forProvider:
  expiration: "2025-06-30T23:59:59Z"
```

### 3. Use Descriptive Names

Use clear, descriptive names and descriptions:

```yaml
forProvider:
  name: "MyApp Production ReadOnly Access"
  description: "Read-only access to production data for MyApp service"
```

### 4. Rotate Credentials Regularly

For long-lived service accounts, implement credential rotation by:

1. Creating a new service account
2. Updating applications to use new credentials  
3. Deleting the old service account

## Troubleshooting

### Common Issues

#### ServiceAccount Creation Fails

```bash
kubectl describe serviceaccount my-serviceaccount
```

Check the events and conditions for error details.

#### Invalid Policy Syntax

ServiceAccount creation will fail with policy validation errors. Ensure your JSON policy is valid:

```bash
echo '{"Version":"2012-10-17",...}' | jq .
```

#### Permission Denied

Verify your provider configuration has sufficient permissions to create service accounts:

- The provider user must have admin privileges
- Check MinIO server logs for detailed error information

#### Credentials Not Available

If the connection secret is not created:

1. Check ServiceAccount status conditions
2. Verify `writeConnectionSecretsToRef` configuration
3. Ensure the target namespace exists

### Status and Conditions

Monitor ServiceAccount status:

```bash
kubectl get serviceaccounts.minio.crossplane.io
kubectl describe serviceaccount my-serviceaccount
```

Key conditions:

- `Ready`: ServiceAccount is created and available
- `Synced`: Controller successfully reconciled the resource

## Examples Repository

Additional examples are available in:

- `examples/minio.crossplane.io_serviceaccount.yaml`
- `samples/minio.crossplane.io_serviceaccount.yaml`
- `test/e2e/serviceaccount/` (E2E test scenarios)

## Related Resources

- [User Resource](./User.md) - Parent user management
- [Policy Resource](./Policy.md) - Canned policy management  
- [Bucket Resource](./Bucket.md) - Bucket access control
- [ProviderConfig](./ProviderConfig.md) - Provider configuration
