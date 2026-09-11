# User

**API Version**: `minio.m.crossplane.io/v1beta1`

Manages MinIO users and their policy attachments. Credentials are published to a connection secret.

## Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `forProvider.userName` | string | no | Username, defaults to metadata.name |
| `forProvider.policies` | []string | no | List of Policy names to attach |

## Example

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: User
metadata:
  name: example-user
  namespace: production
spec:
  forProvider:
    userName: myuser
    policies:
      - example-policy
  writeConnectionSecretToRef:
    name: user-credentials
    namespace: production
  providerConfigRef:
    name: default
```

## Behavior

- **Create**: Creates a new MinIO user
- **Update**: Updates username or attached policies
- **Delete**: Deletes the user and revokes all access keys

## Status Fields

- `status.atProvider.userName` — Confirmed username
- `status.atProvider.policies` — Currently attached policies
- `status.atProvider.status` — User status

## Connection Secret

When `writeConnectionSecretToRef` is specified, the following keys are written:
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
