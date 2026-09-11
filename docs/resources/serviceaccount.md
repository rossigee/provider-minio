# ServiceAccount

**API Version**: `minio.m.crossplane.io/v1beta1`

Manages MinIO service accounts (programmatic access keys bound to a parent user, with optional custom policy and expiry).

## Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `forProvider.name` | string | no | Display name for the service account |
| `forProvider.description` | string | no | Description of the service account |
| `forProvider.targetUser` | string | no | Parent user, defaults to ProviderConfig user |
| `forProvider.accessKey` | string | no | Custom access key (3-128 chars) |
| `forProvider.secretKey` | string | no | Custom secret key (min 8 chars) |
| `forProvider.policy` | string | no | Custom IAM policy JSON |
| `forProvider.expiration` | string | no | RFC3339 expiration timestamp |

## Example

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: ServiceAccount
metadata:
  name: my-app-sa
  namespace: production
spec:
  forProvider:
    name: "MyApp SA"
    description: "Read-only access for MyApp"
    targetUser: example-user
    accessKey: MYACCESSKEY
    policy: |
      {
        "Version": "2012-10-17",
        "Statement": [{ "Effect": "Allow", "Action": ["s3:GetObject"], "Resource": ["arn:aws:s3:::my-bucket/*"] }]
      }
    expiration: "2026-12-31T23:59:59Z"
  writeConnectionSecretToRef:
    name: my-app-credentials
    namespace: production
  providerConfigRef:
    name: default
```

## Behavior

- **Create**: Creates a new service account under the target user
- **Update**: Updates name, description, policy, or expiration
- **Delete**: Deletes the service account and invalidates credentials

## Status Fields

- `status.atProvider.accessKey` — Assigned access key
- `status.atProvider.accountStatus` — Account status
- `status.atProvider.parentUser` — Parent user
- `status.atProvider.policy` — Implied or attached policy
- `status.atProvider.expiration` — Expiration timestamp

## Connection Secret

When `writeConnectionSecretToRef` is specified, the following keys are written:
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
