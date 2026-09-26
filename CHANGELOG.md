# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This file begins at v0.21.4. The published v0.21.3 tag was built from a commit
predating the v0.21.3 release preparation, so none of that work shipped; everything
below is therefore released for the first time in v0.21.4.

## [v0.21.5] - 2026-09-26

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
- Raised the Crossplane minimum version to v2.5.0.
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
