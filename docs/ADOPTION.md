# Resource Adoption Guide

## Overview

The Crossplane MinIO provider supports adopting pre-existing MinIO resources into Crossplane management. This allows you to import resources that were created outside of Crossplane and manage them through Kubernetes CRDs.

## Supported Resources

### ✅ Fully Supported (Adopted & Synced)

- **Buckets** (105/105) - Pre-existing buckets are automatically detected and adopted
- **Users** (4/4) - Existing MinIO users can be adopted
- **Policies** (59/59) - Pre-existing policies are recognized and adopted
- **ServiceAccounts** (58/58) - *Blocked by CRD schema limitation* - namespace field not supported in current Crossplane version

### ⚠️ Partially Supported

- **NotificationConfigurations** (13/13) - *Blocked by MinIO limitation* - webhook notifications not supported by MinIO server

## How Adoption Works

### Observation Phase (`observe.go`)

1. Provider checks if resource exists in MinIO
2. For new resources: Returns `ResourceExists: false` to trigger creation
3. For existing resources: Returns `ResourceExists: false` if not yet claimed
4. Sets appropriate conditions (Creating/Available)

### Creation Phase (`create.go`)

1. Checks if resource already exists in MinIO
2. If found: Marks as adopted and emits adoption event
3. If not found: Creates the resource
4. Sets external-name annotation for future reconciliations

### Update Phase (`update.go`)

1. Reconciles desired state with actual MinIO configuration
2. Updates resources if spec differs from current state

### Delete Phase (`delete.go`)

1. Removes resource from MinIO when CRD is deleted
2. Handles cleanup properly

## Implementation Details

### Bucket Adoption

```go
// observe.go: Return ResourceExists: false for existing buckets without lock annotation
if exists {
    return managed.ExternalObservation{ResourceExists: false}, nil
}

// create.go: Set lock annotation on adoption
meta.SetAnnotation(bucket, lockAnnotation, "claimed")
```

### User Adoption

```go
// observe.go: Check if user exists
if userExists {
    return managed.ExternalObservation{ResourceExists: false}, nil
}

// create.go: Check for existing user and adopt
if userExists(ctx, username) {
    emitAdoptionEvent(user)
    return managed.ExternalCreation{}, nil
}
```

### Policy Adoption

```go
// observer.go: Allow adoption without PolicyCreatedAnnotationKey
if policyExists && allowAdoption {
    return managed.ExternalObservation{ResourceExists: false}, nil
}

// creator.go: Detect existing policies
if policyExists(ctx, policyName) {
    emitAdoptionEvent(policy)
    return managed.ExternalCreation{}, nil
}
```

## Usage

### Adopt a Pre-existing Bucket

1. Create a Crossplane CRD that matches your existing MinIO resource:

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: Bucket
metadata:
  name: my-existing-bucket
  namespace: minio-resources
spec:
  forProvider:
    bucketName: my-existing-bucket
  providerConfigRef:
    kind: ProviderConfig
    name: minio-provider-config
```

1. Apply it to your cluster:

```bash
kubectl apply -f bucket.yaml
```

1. Watch the adoption:

```bash
kubectl get buckets.minio.m.crossplane.io my-existing-bucket -w
```

The resource will automatically be claimed by Crossplane and marked as `SYNCED=True`.

### Adopt a Pre-existing User

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: User
metadata:
  name: my-existing-user
  namespace: minio-resources
spec:
  forProvider:
    userName: my-existing-user
  providerConfigRef:
    kind: ProviderConfig
    name: minio-provider-config
```

### Adopt a Pre-existing Policy

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: Policy
metadata:
  name: my-existing-policy
  namespace: minio-resources
spec:
  forProvider:
    policyName: my-existing-policy
  providerConfigRef:
    kind: ProviderConfig
    name: minio-provider-config
```

## Adoption Events

Successful adoptions emit Kubernetes events:

```bash
kubectl describe bucket my-existing-bucket
# Events:
#   Type    Reason   Age   Message
#   ----    ------   ---   -------
#   Normal  Adopted  2m    Adopted existing bucket: my-existing-bucket
```

## Limitations

### ServiceAccounts

- ⚠️ **Blocked by CRD Schema**: The `namespace` field in `writeConnectionSecretToRef` cannot be added to the CRD due to Crossplane framework constraints
- Requires Crossplane v2 with improved CRD management
- Workaround: Use direct MinIO API until CRD schema issue is resolved

### NotificationConfigurations

- ⚠️ **Blocked by MinIO**: The test MinIO server does not support webhook notifications via the Topic/Cloud Function API
- Adoption logic is implemented but cannot work without MinIO support
- Consider upgrading MinIO or configuring webhook support separately

## Testing

Adoption is tested with integration tests:

```bash
# Run bucket adoption tests
go test ./operator/bucket -run TestAdoption

# Run user adoption tests
go test ./operator/user -run TestAdoption

# Run policy adoption tests
go test ./operator/policy -run TestAdoption
```

## Troubleshooting

### Resource not syncing after adoption

1. Check if resource exists in MinIO:

```bash
# For buckets
mc ls minio-backups/my-bucket/

# For users
mc admin user list minio-backups

# For policies
mc admin policy list minio-backups
```

1. Check for events:

```bash
kubectl describe <resource-type> <resource-name>
```

1. Check provider logs:

```bash
kubectl logs -n crossplane-system deployment/provider-minio-* -f
```

### Adoption failed with "cannot create"

- Ensure the resource actually exists in MinIO
- Check provider credentials with `ProviderConfig`
- Verify MinIO server supports the resource type (e.g., webhooks for NotificationConfigurations)

## Future Improvements

1. **ServiceAccount CRD Schema**: Update Crossplane to support complex nested schemas
2. **NotificationConfiguration**: Wait for MinIO webhook support or implement alternative notification types
3. **Automatic Adoption**: Scan MinIO and auto-create CRDs for existing resources
4. **Adoption Validation**: Comprehensive pre-adoption checks to ensure compatibility

## See Also

- [Crossplane Documentation](https://docs.crossplane.io/)
- [MinIO Management API](https://docs.min.io/minio/baremetal/reference/minio-mc/mc-admin/)
