# Provider MinIO Documentation

A Crossplane v2 provider for managing MinIO/S3 resources with complete namespace isolation for multi-tenancy.

## Quick Links

- [Getting Started](getting-started.md) — Installation and first resource
- [Configuration](configuration.md) — Authentication and connection setup
- [Development](development.md) — Building, testing, and contributing

## Resource Documentation

### Core Resources

| Resource | API Group | Description |
|----------|-----------|-------------|
| [Bucket](resources/bucket.md) | `minio.m.crossplane.io/v1beta1` | S3 bucket management |
| [Policy](resources/policy.md) | `minio.m.crossplane.io/v1beta1` | IAM access policies |
| [User](resources/user.md) | `minio.m.crossplane.io/v1beta1` | User management with policy attachments |
| [ServiceAccount](resources/serviceaccount.md) | `minio.m.crossplane.io/v1beta1` | Programmatic access keys |
| [NotificationConfiguration](resources/notificationconfiguration.md) | `minio.m.crossplane.io/v1beta1` | Bucket notifications (webhook, SQS, SNS) |
| ProviderConfig | `minio.m.crossplane.io/v1beta1` | Provider credentials (cluster-scoped) |

## API Coverage Gaps

MinIO/S3 API surface not yet modeled: bucket versioning/SSE/object-lock settings as dedicated fields, replication rules, lifecycle/expiry rules, IAM groups and service-account policy attachments beyond user policies, bucket event filtering beyond notification targets, and tier/ILM transition rules.

## API Reference

For comprehensive API documentation with all fields and examples, see [API Reference](API.md).
