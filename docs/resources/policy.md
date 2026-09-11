# Policy

**API Version**: `minio.m.crossplane.io/v1beta1`

Manages MinIO access policies (IAM JSON documents).

## Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `forProvider.allowBucket` | string | no* | Simple policy allowing all operations on bucket |
| `forProvider.rawPolicy` | string | no* | Full S3 policy JSON |

*Either allowBucket or rawPolicy is required (mutually exclusive).

## Example

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: Policy
metadata:
  name: example-policy
  namespace: production
spec:
  forProvider:
    allowBucket: my-bucket
  providerConfigRef:
    name: default
```

Full JSON policy example:

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: Policy
metadata:
  name: full-policy
  namespace: production
spec:
  forProvider:
    rawPolicy: |
      {
        "Version": "2012-10-17",
        "Statement": [{ "Effect": "Allow", "Action": ["s3:GetObject"], "Resource": ["arn:aws:s3:::my-bucket/*"] }]
      }
  providerConfigRef:
    name: default
```

## Behavior

- **Create**: Creates a new named policy
- **Update**: Updates policy with new JSON
- **Delete**: Deletes the policy from MinIO

## Status Fields

- `status.atProvider.policy` — Rendered policy JSON
