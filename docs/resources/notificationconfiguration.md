# NotificationConfiguration

**API Version**: `minio.m.crossplane.io/v1beta1`

Configures MinIO bucket notifications (webhook, SQS, SNS) per S3 event.

## Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `forProvider.bucketName` | string | yes | Target bucket name |
| `forProvider.events` | []string | yes | List of S3 events (min 1) |
| `forProvider.webhookConfiguration` | object | no* | Webhook notification config |
| `forProvider.queueConfiguration` | object | no* | SQS queue notification config |
| `forProvider.topicConfiguration` | object | no* | SNS topic notification config |
| `forProvider.filter.key.filterRules` | []object | no | Prefix/suffix filters |

*At least one of webhook/queue/topic configuration is recommended.

## Example

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: NotificationConfiguration
metadata:
  name: my-bucket-webhook
  namespace: production
spec:
  forProvider:
    bucketName: my-bucket
    events: ["s3:ObjectCreated:*"]
    webhookConfiguration:
      id: webhook-1
      endpoint: https://hooks.example.com/minio
      authToken: secret-token
    filter:
      key:
        filterRules:
          - name: prefix
            value: uploads/
  providerConfigRef:
    name: default
```

## Behavior

- **Create**: Registers notification configuration on the bucket
- **Update**: Updates events or notification targets
- **Delete**: Removes notification configuration from bucket

## Status Fields

- `status.atProvider.configurationId` — Assigned configuration ID
- `status.atProvider.bucketName` — Confirmed bucket name
- `status.atProvider.lastUpdated` — Last update timestamp
