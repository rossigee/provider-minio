# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This file begins at v0.21.4. The published v0.21.3 tag was built from a commit
predating the v0.21.3 release preparation, so none of that work shipped; everything
below is therefore released for the first time in v0.21.4.

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

### Docs

- Corrected obsolete `deletionPolicy`, plural connection-secret field and ProviderConfig name references, and removed descriptions of the samples as legacy v1-style.
