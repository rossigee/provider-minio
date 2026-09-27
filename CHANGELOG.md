# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This file begins at v0.21.4. The published v0.21.3 tag was built from a commit
predating the v0.21.3 release preparation, so none of that work shipped; everything
below is therefore released for the first time in v0.21.4.

## [v0.21.6] - 2026-09-27

### Fixed in v0.21.6

- Restored `make test-e2e`, which reported `No rule to make target 'test-e2e'`. The
  `-include test/local.mk` line was removed in 1554ce1 and never restored, so the 26 files
  under `test/e2e/` could not be executed by any command.
- Rebuilt `test/local.mk` on the vendored kind machinery: `controlplane.up` brings up the
  cluster and Crossplane, and `local.xpkg.deploy.provider.$(PROJECT_NAME)` side-loads the
  locally built provider image. This removes the in-cluster docker-registry, `mirror-setup`
  and `package-push-local` flow that existed only so Crossplane could pull an image.
- MinIO now runs in-cluster with no ingress, and the end to end suite is given its own
  kubeconfig. The suite previously drove its own object uploads through an ingress at
  `minio.127.0.0.1.nip.io`, which requires public DNS and ingress-nginx and so could never
  run on a CI runner. The suite is also no longer at risk of acting on whatever cluster an
  ambient `KUBECONFIG` happens to point at.
- The end to end suite is **not run on pull requests or master pushes**, only on manual
  dispatch. MinIO's published images are no longer pullable anonymously:
  `quay.io/minio/minio` and `docker.io/minio/minio` both return `401` with a valid anonymous
  token, `ghcr.io/minio/minio` returns `403`, and `bitnami/minio` has been removed, with no
  public mirror available. The suite therefore cannot complete on a stock runner, and
  running it on every push would mean a permanently red job. `MINIO_IMAGE_REPOSITORY` and
  `MINIO_IMAGE_TAG` select the image, so pointing them at an internal mirror is all that
  stands between this and a green run.

### Known in v0.21.6

- **This release cannot be installed until a Crossplane v2.5.0 artifact is available.** The
  package requires `crossplane.version: ">=v2.5.0"`, and no such artifact is published:
  the latest upstream release is v2.4.2, `charts.crossplane.io/stable` tops out at v2.4.2
  across 161 chart versions, and there is no v2.5.0 or v2.5.0-rc.0 image on
  `docker.io/crossplane/crossplane`, `ghcr.io/crossplane/crossplane` or
  `xpkg.crossplane.io/crossplane/crossplane`. The `rossigee/crossplane` fork carries the
  upstream `v2.5.0-rc.0` tag but publishes no release and no image for it. Installing this
  package against a released Crossplane fails with
  `incompatible Crossplane version: package is not compatible with Crossplane version`.
  The v2.5.0 requirement is retained deliberately; it must not be relaxed to v2.4.2.
  Resolving it requires a published Crossplane v2.5.0 chart and image, or an override of
  `CROSSPLANE_CHART_REPO` and `CROSSPLANE_VERSION` for the control plane under test.

## [v0.21.5] - 2026-09-27

### Fixed in v0.21.5

- **ServiceAccount connection secrets are now written.** `ServiceAccountSpec` re-declared
  a `writeConnectionSecretToRef` field that shadowed the one inherited from the embedded
  `ManagedResourceSpec`, giving two Go fields the same JSON tag. The outer field won for
  JSON, so a manifest populated it, but the generated
  `GetWriteConnectionSecretToReference` accessor reads the embedded field, which therefore
  always returned `nil`. The managed reconciler had no destination for the connection
  details and never created the Secret, so a ServiceAccount could not be used to obtain
  credentials. The shadowing field and its custom `SecretReferenceWithNamespace` type have
  been removed; ServiceAccount now uses the standard name-only reference like every other
  managed resource in this provider, and the Secret is written to the ServiceAccount's own
  namespace.
- Guarded two latent nil-map writes in `policy/create.go` and `user/create.go` that would
  panic on an object with no annotations. These are not reachable today, because the
  managed reconciler sets a create-pending annotation before calling the external client,
  but they now match the guarded pattern already used in `bucket`.
- Removed an unused MinIO admin client from the NotificationConfiguration controller. It had
  no callers, but it was constructed on every reconcile and required valid credentials, so
  it could fail the connection for a client nothing read.

### Testing in v0.21.5

- `make test` no longer silently skips `cmd/` and `internal/`. The `GO_SUBDIRS` assignment
  used `+=` and was evaluated before the build system declares its default, so `cmd/provider`
  and `internal/clients` tests have never run in CI.
- Made the MinIO client substitutable behind narrow per-resource interfaces, allowing the
  controllers to be tested with fakes. All five `connector.go` files previously had no
  coverage and could not be tested at all. Per-package coverage: bucket 22.4% to 52.5%,
  policy 22.9% to 62.8%, user 17.6% to 65.7%, serviceaccount 13.4% to 51.4%,
  notificationconfiguration 6.2% to 23.7%.
- Removed package-level function variables in `operator/bucket` that existed only so tests
  could monkey-patch them. One was overwritten without being restored and another was never
  stubbed, so state leaked between tests.
- Replaced five no-op `setup_test.go` files, and a test that asserted `ptr.To(true)` was
  true and so passed even if watch bookmarks were disabled.

### Breaking in v0.21.5

- `spec.writeConnectionSecretToRef` for `ServiceAccount` no longer accepts a `namespace`
  field; only `name` is permitted, consistent with the other managed resources. The field
  was never functional, so no working configuration depended on it. The Secret is written to
  the ServiceAccount's own namespace. `spec.forProvider.credentialsSecretRef` is unchanged
  and still takes a name and namespace, because it references a Secret elsewhere.

## [v0.21.4] - 2026-09-26

### Changed

- Standardized tag-only release publishing for both supported Linux architectures.
- Ensured cross-architecture image builds use target-specific binaries.
- Regenerated the sample manifests under the `minio.m.crossplane.io/v1beta1` API group and renamed the files to match.

### Fixed

- Corrected package metadata to use `package/crossplane.yaml` as the Provider package input, removed the duplicate `package/package.yaml` compatibility file, and enabled the runtime image to be embedded in the xpkg while retaining the five generated namespaced CRDs.
- Removed the duplicated secret permissions from the package RBAC rules.
- Added the required `spec.providerConfigRef.kind` to every managed resource, sample and example. The Crossplane v2 CRDs default this field to `ClusterProviderConfig` only when `providerConfigRef` is absent, so a reference that set `name` alone was rejected by the API server. This also broke `make install-samples`.
- Replaced the removed `spec.deletionPolicy` with `spec.managementPolicies` in the e2e assertions.
- Renamed `writeConnectionSecretsToRef` to `writeConnectionSecretToRef`. The plural form was being silently pruned, so connection secrets were never written.
- Removed the invalid `namespace` field from name-only local secret references, retaining it where the API requires both a name and a namespace.
- Corrected the ServiceAccount manifests to reference the `provider-config` ProviderConfig that the suites and samples actually create.
- Updated the e2e and sample manifests to the `minio.m.crossplane.io/v1beta1` API group and lock annotation group.
- Removed dead `forProvider` fields (`zone`, `versioning` and `public`).
- Rewrote `generate_sample.go`, which had never compiled. It now regenerates the five structured samples idempotently and no longer deletes the hand-written TLS and ServiceAccount samples.
- Fixed ServiceAccount connection secrets never being written. `ServiceAccountSpec` re-declared a `writeConnectionSecretToRef` field that shadowed the one inherited from the embedded `ManagedResourceSpec`, giving two Go fields the same JSON tag. The generated `GetWriteConnectionSecretToReference` accessor reads the embedded field, which therefore stayed `nil`, so the managed reconciler discarded the connection details and never created the Secret. The shadowing field and its custom `SecretReferenceWithNamespace` type have been removed; ServiceAccount now uses the standard name-only reference like every other managed resource in this provider, and the Secret is written to the ServiceAccount's own namespace. As a side effect the `namespace` field is no longer accepted under `writeConnectionSecretToRef` for ServiceAccount.

### Testing

- Made the MinIO client substitutable behind narrow per-resource interfaces, so controllers can be tested with fakes. All five `connector.go` files previously had 0% coverage and could not be tested at all, because each client struct held a concrete `*minio.Client` or `*madmin.AdminClient`.
- Removed the package-level function variables in `operator/bucket` that existed only so tests could monkey-patch them. One was overwritten without being restored and another was never stubbed, so state leaked between tests.
- `make test` no longer silently skips `cmd/` and `internal/`. The `GO_SUBDIRS` assignment used `+=` before `golang.mk` declared its default, so the default never applied.
- Replaced five no-op `setup_test.go` files and a tautological `main_test.go` that asserted `ptr.To(true)` was true, with tests that assert real behaviour and fail when the behaviour regresses.

### Docs

- Corrected obsolete `deletionPolicy`, plural connection-secret field and ProviderConfig name references, and removed descriptions of the samples as legacy v1-style.
