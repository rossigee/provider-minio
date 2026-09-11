# API Reference

This document describes the Custom Resource Definitions (CRDs) provided by `provider-minio`.

Managed resources are **namespaced** (`minio.m.crossplane.io/v1beta1`) and require a cluster-scoped `ProviderConfig` (`minio.crossplane.io/v1`).

## ProviderConfig (Cluster-Scoped)

Configures connection settings for the MinIO provider.

```yaml
apiVersion: minio.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: default
spec:
  minioURL: https://minio.example.com:9000
  credentials:
    source: Secret
    apiSecretRef:
      name: minio-credentials
      namespace: crossplane-system
  tls: {} # optional - see docs/TLS_CONFIGURATION.md
```

Fields:

* `spec.minioURL` (string, required) — MinIO endpoint URL (e.g. `https://minio.example.com:9000` or `http://minio.minio.svc:9000`). Scheme determines `Secure` (`https` = TLS).
* `spec.credentials.source` (enum: `Secret` / `InjectedIdentity` etc.) — see `apis/provider/v1/providerconfig_types.go:27`
* `spec.credentials.apiSecretRef` (`SecretReference`) — secret with keys `accessKey`/`secretKey` **or** `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` depending on controller path (`internal/clients/minio.go:39`, `operator/minioutil/client.go:28`). The canonical test secret uses `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` (`operator/minioutil/client.go:21`).
* `spec.credentials.secretRef` / `CommonCredentialSelectors` — alternative JSON-blob secret reference (used by `internal/clients/minio.go:45` via `secretRef.key`).
* `spec.tls` (`common.TLSConfig` optional) — see `apis/common/common.go:23`.

> Note: `ProviderConfig` is **cluster-scoped** (`apis/provider/v1/providerconfig_types.go:44`). Do not set `namespace`.

---

## Bucket

Manages MinIO S3 buckets.

**Group:** `minio.m.crossplane.io`
**Version:** `v1beta1`
**Scope:** `Namespaced`
**CRD:** `package/crds/minio.m.crossplane.io_buckets.yaml:9`

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: Bucket
metadata:
  name: my-bucket
  namespace: production
spec:
  forProvider:
    bucketName: my-bucket  # optional, defaults to metadata.name
    region: us-east-1      # required, defaults to us-east-1
    bucketDeletionPolicy: DeleteIfEmpty # DeleteIfEmpty | DeleteAll
    policy: |
      {
        "Version": "2012-10-17",
        "Statement": [{ "Effect": "Allow", "Principal": "*", "Action": ["s3:GetObject"], "Resource": ["arn:aws:s3:::my-bucket/*"] }]
      }
    tags:                  # optional map[string]string
      env: production
  providerConfigRef:
    name: default
  deletionPolicy: Delete   # Crossplane: Delete | Orphan
  writeConnectionSecretToRef:
    name: bucket-connection
```

Key fields (`apis/minio/v1beta1/bucket_types.go`):

* `spec.forProvider.bucketName` — defaults to `metadata.name`; immutable.
* `spec.forProvider.region` — required; defaults `us-east-1`; immutable.
* `spec.forProvider.bucketDeletionPolicy` — `DeleteIfEmpty` or `DeleteAll`; if omitted and `spec.deletionPolicy=Orphan`, bucket is orphaned.
* `spec.forProvider.policy` — raw JSON bucket policy (string, optional).
* `spec.forProvider.tags` — S3 bucket tags (nil = unmanaged, empty map = reconcile to empty).
* Status: `status.atProvider.bucketName`, `status.endpoint`, `status.endpointURL`, `status.conditions` (`Ready`, `Synced`).

---

## Policy

Manages MinIO access policies (IAM JSON documents).

**Group:** `minio.m.crossplane.io`
**Version:** `v1beta1`
**Scope:** `Namespaced`
**CRD:** `package/crds/minio.m.crossplane.io_policies.yaml`

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: Policy
metadata:
  name: example-policy
  namespace: production
spec:
  forProvider:
    # Either allowBucket (simple) or rawPolicy (full JSON) — mutually exclusive
    allowBucket: my-bucket
    # rawPolicy: |
    #   {
    #     "Version": "2012-10-17",
    #     "Statement": [{ "Effect": "Allow", "Action": ["s3:GetObject"], "Resource": ["arn:aws:s3:::my-bucket/*"] }]
    #   }
  providerConfigRef:
    name: default
```

Fields (`apis/minio/v1beta1/policy_types.go:46`):

* `spec.forProvider.allowBucket` (string) — simple policy allowing all operations on bucket.
* `spec.forProvider.rawPolicy` (string) — full S3 policy JSON.

Status: `status.atProvider.policy` (rendered JSON).

---

## User

Manages MinIO users and their policy attachments. Credentials are published to a connection secret.

**Group:** `minio.m.crossplane.io`
**Version:** `v1beta1`
**Scope:** `Namespaced`
**CRD:** `package/crds/minio.m.crossplane.io_users.yaml`

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: User
metadata:
  name: example-user
  namespace: production
spec:
  forProvider:
    userName: myuser   # optional, defaults to metadata.name
    policies:          # optional list of Policy names
      - example-policy
  writeConnectionSecretToRef:
    name: user-credentials
    namespace: production
  providerConfigRef:
    name: default
```

Fields (`apis/minio/v1beta1/user_types.go:52`):

* `spec.forProvider.userName` — defaults to `metadata.name`; immutable.
* `spec.forProvider.policies` — list of existing Policy resources to attach.
* `spec.writeConnectionSecretToRef` — local secret reference where `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` are written (optional but recommended).

Status: `status.atProvider.userName`, `status.atProvider.policies`, `status.atProvider.status`.

---

## ServiceAccount

Manages MinIO service accounts (programmatic access keys bound to a parent user, with optional custom policy and expiry).

**Group:** `minio.m.crossplane.io`
**Version:** `v1beta1`
**Scope:** `Namespaced`
**CRD:** `package/crds/minio.m.crossplane.io_serviceaccounts.yaml`
**Full guide:** `docs/ServiceAccount.md`

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
    targetUser: example-user   # optional, defaults to ProviderConfig user
    accessKey: MYACCESSKEY     # optional 3-128 chars, immutable
    secretKey: mysecretkey123  # optional min 8 chars, immutable
    policy: |
      {
        "Version": "2012-10-17",
        "Statement": [{ "Effect": "Allow", "Action": ["s3:GetObject"], "Resource": ["arn:aws:s3:::my-bucket/*"] }]
      }
    expiration: "2026-12-31T23:59:59Z" # optional RFC3339
  writeConnectionSecretToRef:
    name: my-app-credentials
    namespace: production
  providerConfigRef:
    name: default
```

Fields (`apis/minio/v1beta1/serviceaccount_types.go:58`):

* `spec.forProvider.targetUser`, `accessKey`, `secretKey`, `name`, `description`, `policy`, `expiration`
* Status: `status.atProvider.{accessKey,accountStatus,parentUser,impliedPolicy,policy,expiration}`

Connection secret keys: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`.

---

## NotificationConfiguration

Configures MinIO bucket notifications (webhook, SQS, SNS) per S3 event.

**Group:** `minio.m.crossplane.io`
**Version:** `v1beta1`
**Scope:** `Namespaced`
**CRD:** `package/crds/minio.m.crossplane.io_notificationconfigurations.yaml`

```yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: NotificationConfiguration
metadata:
  name: my-bucket-webhook
  namespace: production
spec:
  forProvider:
    bucketName: my-bucket              # required
    events: ["s3:ObjectCreated:*"]     # required, min 1
    webhookConfiguration:              # one of webhook/queue/topic
      id: webhook-1
      endpoint: https://hooks.example.com/minio
      authToken: secret-token
    # queueConfiguration:
    #   id: queue-1
    #   queueArn: arn:aws:sqs:us-east-1:123456789012:my-queue
    # topicConfiguration:
    #   id: topic-1
    #   topicArn: arn:aws:sns:us-east-1:123456789012:my-topic
    filter:
      key:
        filterRules:
          - name: prefix
            value: uploads/
  providerConfigRef:
    name: default
```

Fields (`apis/minio/v1beta1/notificationconfiguration_types.go:41`):

* `spec.forProvider.bucketName` (required)
* `spec.forProvider.events` (required, `[]string`)
* `spec.forProvider.webhookConfiguration` / `queueConfiguration` / `topicConfiguration` — at least one recommended
* `spec.forProvider.filter.key.filterRules[]` — prefix/suffix filters.

Status: `status.atProvider.configurationId`, `bucketName`, `lastUpdated`.

---

## Common Fields

All managed resources embed `xpv1.ManagedResourceSpec`:

* `spec.providerConfigRef` — reference to `ProviderConfig` (default `name: default`, `kind: ProviderConfig` per `package/crds/minio.m.crossplane.io_buckets.yaml:135`)
* `spec.managementPolicies` — `Observe|Create|Update|Delete|LateInitialize|*` (default `*`)
* `spec.deletionPolicy` — `Delete` | `Orphan`
* `spec.writeConnectionSecretToRef` / `spec.writeConnectionSecretsToRef` — where provider writes connection details

Status:

* `status.conditions` — `Ready`, `Synced` (see `xpkube` conditions)
* `status.atProvider` — provider-observed state per kind

---

## Examples

Hand-written namespaced examples: `examples/v2/bucket-namespaced.yaml`, `examples/v2/user-namespaced.yaml`, `examples/minio.crossplane.io_serviceaccount.yaml` (note filename is legacy; content is v1beta1).

Generated legacy samples (cluster-style but now stale, regenerated via `go generate`): `samples/minio.crossplane.io_bucket.yaml` etc. — prefer `examples/v2/` for v1beta1.

See also: `docs/CONFIGURATION.md` (ProviderConfig + TLS), `docs/ServiceAccount.md`, `docs/TLS_CONFIGURATION.md`.
