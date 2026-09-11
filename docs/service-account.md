# ServiceAccount Resource

The `ServiceAccount` resource provides declarative management of MinIO service accounts through Crossplane. Service accounts enable programmatic access to MinIO with specific permissions and time-bound credentials.

> **APIVersion:** `minio.m.crossplane.io/v1beta1` namespaced (`apis/minio/v1beta1/serviceaccount_types.go:16`).
> **CRD:** `package/crds/minio.m.crossplane.io_serviceaccounts.yaml`.
> For `ProviderConfig` and TLS, see `docs/CONFIGURATION.md` and `docs/TLS_CONFIGURATION.md`.

## Overview

ServiceAccounts in MinIO are specialized credentials that:

* Belong to a parent user
* Can have their own IAM policies
* Support expiration dates for enhanced security
* Generate access/secret key pairs for authentication
* Enable fine-grained permission control

## Resource Specification

### Basic ServiceAccount (Namespaced)

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: ServiceAccount
metadata:
  name: my-app-sa
  namespace: production
spec:
  providerConfigRef:
    name: default
  forProvider:
    name: "Application Service Account"
    description: "Service account for my application"
  writeConnectionSecretToRef:
    name: my-app-credentials
    namespace: production
```

### ServiceAccount with Custom Policy

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: ServiceAccount
metadata:
  name: restricted-sa
  namespace: production
spec:
  providerConfigRef:
    name: default
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
  writeConnectionSecretToRef:
    name: restricted-credentials
    namespace: production
```

### ServiceAccount with Expiration

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: ServiceAccount
metadata:
  name: temporary-sa
  namespace: production
spec:
  providerConfigRef:
    name: default
  forProvider:
    name: "Temporary Service Account"
    description: "Time-limited access for maintenance tasks"
    expiration: "2026-12-31T23:59:59Z"
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
  writeConnectionSecretToRef:
    name: temp-credentials
    namespace: production
```

### ServiceAccount with Custom Credentials

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: ServiceAccount
metadata:
  name: custom-sa
  namespace: production
spec:
  providerConfigRef:
    name: default
  forProvider:
    name: "Custom Credentials Service Account"
    description: "Service account with predefined access keys"
    accessKey: "CUSTOM_ACCESS_KEY"
    secretKey: "custom-secret-key-minimum-8-chars"
    targetUser: "specific-parent-user"
  writeConnectionSecretToRef:
    name: custom-credentials
    namespace: production
```

## Field Reference

### ServiceAccountParameters

`apis/minio/v1beta1/serviceaccount_types.go:58`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | No | Human-readable name for the service account |
| `description` | string | No | Description of the service account's purpose |
| `accessKey` | string | No | Custom access key (3-128 characters). If not specified, MinIO generates one. Immutable. |
| `secretKey` | string | No | Custom secret key (minimum 8 characters). If not specified, MinIO generates one. Immutable. |
| `targetUser` | string | No | Parent user for the service account. Defaults to ProviderConfig user |
| `policy` | string | No | JSON IAM policy document. If not specified, inherits parent user policies |
| `expiration` | string (`metav1.Time`) | No | RFC 3339 timestamp when the service account expires |
| `writeConnectionSecretToRef` | `SecretReference` | No | Secret where connection details are written |

> Use `spec.writeConnectionSecretToRef` (singular) as defined in `serviceaccount_types.go:97`. Older docs used `writeConnectionSecretsToRef` (plural) — prefer singular.

### ServiceAccountProviderStatus

| Field | Type | Description |
|-------|------|-------------|
| `accessKey` | string | The actual access key ID created in MinIO |
| `accountStatus` | string | Status of the service account (enabled/disabled) |
| `parentUser` | string | The user that owns this service account |
| `impliedPolicy` | boolean | Whether the policy is inherited from the parent user |
| `policy` | string | The actual policy document applied to the service account |
| `expiration` | `metav1.Time` | When the service account expires (if set) |

## Connection Secrets

ServiceAccounts publish connection credentials to Kubernetes secrets:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-app-credentials
  namespace: production
type: Opaque
data:
  AWS_ACCESS_KEY_ID: <base64-encoded-access-key>
  AWS_SECRET_ACCESS_KEY: <base64-encoded-secret-key>
```

Consume from applications:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: production
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

```yaml
policy: |
  {
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Action": ["s3:GetObject"],
        "Resource": ["arn:aws:s3:::specific-bucket/specific-prefix/*"]
      }
    ]
  }
```

### 2. Set Expiration Dates

For temporary access, always set `expiration`:

```yaml
forProvider:
  expiration: "2026-06-30T23:59:59Z"
```

### 3. Use Descriptive Names

```yaml
forProvider:
  name: "MyApp Production ReadOnly Access"
  description: "Read-only access to production data for MyApp service"
```

### 4. Rotate Credentials Regularly

1. Create a new service account
2. Update applications to use new credentials
3. Delete the old service account

## Troubleshooting

### ServiceAccount Creation Fails

```bash
kubectl describe serviceaccount my-app-sa -n production
kubectl get events -n production --field-selector involvedObject.name=my-app-sa
```

### Invalid Policy Syntax

```bash
echo '{"Version":"2012-10-17",...}' | jq .
```

### Permission Denied

* The ProviderConfig user must have admin privileges
* Check MinIO server logs

### Credentials Not Available

1. Check `status.conditions` (`Ready`, `Synced`)
2. Verify `writeConnectionSecretToRef` namespace exists
3. Ensure `targetUser` exists if specified

### Status and Conditions

Monitor status:

```bash
kubectl get serviceaccounts -A
kubectl get serviceaccount my-app-sa -n production -o yaml
kubectl describe serviceaccount my-app-sa -n production
```

Note: group is `minio.m.crossplane.io`, not `minio.crossplane.io`:

```bash
kubectl get serviceaccounts.minio.m.crossplane.io -A
```

## Examples

* `examples/minio.crossplane.io_serviceaccount.yaml` (filename legacy, content is `minio.m.crossplane.io/v1beta1`)
* `examples/v2/` — canonical v1beta1 examples
* `samples/` — generated legacy samples (see `generate_sample.go`)
* `test/e2e/serviceaccount/` — E2E scenarios

## Related Resources

* `docs/API.md` — all resources
* `docs/CONFIGURATION.md` — ProviderConfig
* `docs/TLS_CONFIGURATION.md` — TLS
* `docs/GETTING_STARTED.md` — end-to-end flow
