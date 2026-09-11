# Bucket

**API Version**: `minio.m.crossplane.io/v1beta1`

Manages MinIO S3 buckets.

## Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `forProvider.bucketName` | string | no | Bucket name, defaults to metadata.name |
| `forProvider.region` | string | no | AWS region, defaults to us-east-1 |
| `forProvider.bucketDeletionPolicy` | string | no | DeleteIfEmpty or DeleteAll |
| `forProvider.policy` | string | no | Raw JSON bucket policy |
| `forProvider.tags` | map[string]string | no | S3 bucket tags |

## Example

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: Bucket
metadata:
  name: my-bucket
  namespace: production
spec:
  forProvider:
    bucketName: my-bucket
    region: us-east-1
    bucketDeletionPolicy: DeleteIfEmpty
    policy: |
      {
        "Version": "2012-10-17",
        "Statement": [{ "Effect": "Allow", "Principal": "*", "Action": ["s3:GetObject"], "Resource": ["arn:aws:s3:::my-bucket/*"] }]
      }
    tags:
      env: production
  providerConfigRef:
    name: default
  deletionPolicy: Delete
```

## Behavior

- **Create**: Creates a new S3 bucket at the specified location
- **Update**: Updates bucket policy, tags, or region (region change may recreate bucket)
- **Delete**: Deletes bucket based on bucketDeletionPolicy (DeleteIfEmpty or DeleteAll)

## Status Fields

- `status.atProvider.bucketName` — Confirmed bucket name
- `status.endpoint` — Bucket endpoint
- `status.endpointURL` — Full URL to bucket
- `status.conditions` — Ready and Synced conditions
