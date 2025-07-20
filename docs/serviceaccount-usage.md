# MinIO ServiceAccount Usage Guide

## Overview
The MinIO provider v0.4.4 adds support for managing MinIO service accounts through Crossplane. ServiceAccounts provide a way to create access credentials that inherit permissions from a parent user.

## Prerequisites
- MinIO provider v0.4.4 or later installed
- A MinIO instance with admin credentials configured in a ProviderConfig
- An existing MinIO user to act as the parent

## Basic Usage

### 1. Create a ServiceAccount
```yaml
apiVersion: minio.crossplane.io/v1
kind: ServiceAccount
metadata:
  name: app-serviceaccount
spec:
  forProvider:
    parentUser: "existing-minio-user"
    serviceAccountName: "app-sa"  # Optional, auto-generated if not specified
    policies:
      - "readwrite"
      - "diagnostics"
    description: "Service account for application XYZ"
  providerConfigRef:
    name: default
  writeConnectionSecretToRef:
    name: app-sa-credentials
    namespace: default
```

### 2. Access Credentials
The ServiceAccount controller automatically creates a Kubernetes secret with the credentials:

```bash
kubectl get secret app-sa-credentials -o jsonpath='{.data.accessKey}' | base64 -d
kubectl get secret app-sa-credentials -o jsonpath='{.data.secretKey}' | base64 -d
```

### 3. Use in Applications
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: minio-app
spec:
  template:
    spec:
      containers:
      - name: app
        env:
        - name: MINIO_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: app-sa-credentials
              key: accessKey
        - name: MINIO_SECRET_KEY
          valueFrom:
            secretKeyRef:
              name: app-sa-credentials
              key: secretKey
```

## Advanced Features

### Temporary ServiceAccounts
Create service accounts that expire automatically:

```yaml
spec:
  forProvider:
    parentUser: "admin"
    expiry: "2025-12-31T23:59:59Z"
    description: "Temporary access until end of year"
```

### Policy Management
ServiceAccounts inherit their parent user's policies but can have additional policies attached:

```yaml
spec:
  forProvider:
    parentUser: "restricted-user"
    policies:
      - "readonly"  # Additional restriction
      - "bucket-specific-policy"
```

## Lifecycle Management

### Updates
- Policies can be updated after creation
- Description can be changed
- Parent user is immutable (webhook validation enforces this)

### Deletion
When a ServiceAccount resource is deleted:
1. The MinIO service account is removed
2. The Kubernetes secret is automatically cleaned up
3. Any applications using those credentials will lose access

## Troubleshooting

### Check ServiceAccount Status
```bash
kubectl describe serviceaccount app-serviceaccount
```

### View Events
```bash
kubectl get events --field-selector involvedObject.name=app-serviceaccount
```

### Common Issues
1. **Parent user not found**: Ensure the parentUser exists in MinIO
2. **Policy errors**: Verify policy names are valid in MinIO
3. **Connection issues**: Check ProviderConfig has correct MinIO endpoint/credentials

## Security Best Practices
1. Use least-privilege policies
2. Set expiry dates for temporary access
3. Rotate service accounts periodically
4. Monitor service account usage in MinIO logs
5. Use separate namespaces for credential secrets