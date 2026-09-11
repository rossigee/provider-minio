# Getting Started

Minimal end-to-end flow for `provider-minio` with a local MinIO instance.

## 0. Prerequisites

* Completed `docs/INSTALLATION.md` (Crossplane, Provider, ProviderConfig)
* `kubectl` access to the cluster

## 1. Create a Bucket

```yaml
# bucket.yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: Bucket
metadata:
  name: my-bucket
  namespace: default
spec:
  forProvider:
    bucketName: my-bucket   # optional; defaults to metadata.name
    region: us-east-1
    bucketDeletionPolicy: DeleteIfEmpty
  providerConfigRef:
    name: default
```

```bash
kubectl apply -f bucket.yaml
kubectl get bucket my-bucket -n default
kubectl wait --for=condition=Ready bucket/my-bucket -n default --timeout=60s
kubectl describe bucket my-bucket -n default
```

Expected status (`package/crds/minio.m.crossplane.io_buckets.yaml:168`):

* `status.conditions[Ready]=True`, `Synced=True`
* `status.atProvider.bucketName` populated

## 2. Create a Policy

```yaml
# policy.yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: Policy
metadata:
  name: read-only-bucket
  namespace: default
spec:
  forProvider:
    rawPolicy: |
      {
        "Version": "2012-10-17",
        "Statement": [{
          "Effect": "Allow",
          "Action": ["s3:GetObject", "s3:ListBucket"],
          "Resource": ["arn:aws:s3:::my-bucket", "arn:aws:s3:::my-bucket/*"]
        }]
      }
  providerConfigRef:
    name: default
```

```bash
kubectl apply -f policy.yaml
kubectl get policy read-only-bucket -n default
```

## 3. Create a User

```yaml
# user.yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: User
metadata:
  name: app-user
  namespace: default
spec:
  forProvider:
    userName: app-user
    policies:
      - read-only-bucket
  writeConnectionSecretToRef:
    name: app-user-credentials
    namespace: default
  providerConfigRef:
    name: default
```

```bash
kubectl apply -f user.yaml
kubectl get users -n default
kubectl get secret app-user-credentials -n default -o yaml
```

## 4. Create a ServiceAccount

See `docs/ServiceAccount.md` for full details.

```yaml
# serviceaccount.yaml
apiVersion: minio.m.crossplane.io/v1beta1
kind: ServiceAccount
metadata:
  name: app-sa
  namespace: default
spec:
  forProvider:
    description: "App read-only SA"
    policy: |
      {
        "Version": "2012-10-17",
        "Statement": [{ "Effect": "Allow", "Action": ["s3:GetObject"], "Resource": ["arn:aws:s3:::my-bucket/*"] }]
      }
  writeConnectionSecretToRef:
    name: app-sa-credentials
    namespace: default
  providerConfigRef:
    name: default
```

```bash
kubectl apply -f serviceaccount.yaml
kubectl get serviceaccounts -n default
kubectl get secret app-sa-credentials -n default -o yaml
```

## 5. Verify in MinIO

```bash
# If MinIO is port-forwarded locally
mc alias set local http://localhost:9000 minioadmin minioadmin
mc ls local
mc admin user list local
mc admin policy list local
```

## 6. Clean Up

```bash
kubectl delete serviceaccount app-sa -n default
kubectl delete user app-user -n default
kubectl delete policy read-only-bucket -n default
kubectl delete bucket my-bucket -n default
```

`Bucket.spec.forProvider.bucketDeletionPolicy: DeleteIfEmpty` vs `DeleteAll` controls whether non-empty buckets block deletion; `spec.deletionPolicy: Orphan` skips deletion entirely (`apis/minio/v1beta1/bucket_types.go:11`).

## Examples

* `examples/v2/bucket-namespaced.yaml` — canonical Bucket example
* `examples/v2/user-namespaced.yaml` — canonical User example
* `examples/minio.crossplane.io_serviceaccount.yaml` — ServiceAccount (legacy filename, v1beta1 content)
* `samples/` — generated samples (see `generate_sample.go:10`)
