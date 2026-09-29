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
  dispatch, because a 2.5.0 Crossplane image has to be supplied (see Known below) and the
  full run builds a provider image. `MINIO_IMAGE_REPOSITORY` and `MINIO_IMAGE_TAG` select
  the MinIO image, defaulting to the anonymously pullable `pgsty/minio` and `pgsty/mc`
  mirrors, because MinIO's own `quay.io/minio/minio`, `quay.io/minio/mc` and
  `docker.io/minio/minio` all return `401` with a valid anonymous token and `bitnami/minio`
  has been removed.
- Skips the MinIO chart's post-install hook with `--no-hooks`. That hook runs
  `/bin/sh /config/add-user`, which reads `/config/rootUser`, but the chart only mounts a
  secret there when `existingSecret` and `existingSecretKey` are set, and `existingSecretKey`
  is not a value this chart version defines. The job therefore aborts on
  `cat: /config/rootUser: No such file or directory`, and because `helm --wait` covers hooks
  the install fails even though the MinIO server is healthy. The hook only creates a
  convenience user; the suite authenticates with `rootUser`/`rootPassword`, which reach the
  server through the chart's own secret.

### Known in v0.21.6

- **The end to end suite now runs and passes.** It had never been executed in this
  project's history, and doing so found a real defect immediately.
- **Fixed a ServiceAccount update loop.** The controller compared the inline policy as a raw
  string against the policy MinIO returns, but MinIO re-serialises it and sorts the `Action`
  and `Resource` arrays, so the comparison never matched. Every reconcile therefore issued an
  `UpdateServiceAccount` call and the resource never converged to `UpToDate`: it sat at
  `Updating` forever, calling MinIO once per poll interval. The policies are now compared as
  canonicalised JSON, with object keys and array elements sorted, so ordering differences no
  longer register as a change. A ServiceAccount with a `policy` set now reaches `Available`
  in about 20 seconds.
- **Fixed ServiceAccount being reported `Disabled`.** `madmin.AccountEnabled` is `"enabled"`,
  which is what the admin API reports for users, but `InfoServiceAccountResp` reports service
  account status as `"on"`. The comparison only accepted `"enabled"`, so no service account
  was ever `Available`. Both spellings are now accepted.
- Repaired the end to end test files, which had never been validated against a running system.
  The bucket and policy asserts expected `status.endpoint` and `status.endpointURL`, fields
  the provider has never written and which do not exist on any status type. The serviceaccount
  asserts expected a secret of type `Opaque` with empty values, whereas the controller writes
  `connection.crossplane.io/v1alpha1` with both keys populated, and expected
  `accountStatus: enabled` rather than `on`; `on` additionally has to be quoted because YAML
  reads it as a boolean. The access pod used the un-pullable `minio/mc` image and pointed at
  `minio.default.svc`, while the service is in the `minio` namespace. The connection secret is
  now checked by a script step rather than an exact match, since the values are generated.
  `TestStep` and `TestAssert` are kept in separate files because kuttl does not accept them in
  one document, and command steps are scripts because kuttl executes them word by word rather
  than through a shell.
- The Go dependencies now track the `develop` branch of the Crossplane forks rather than
  release tags, so the provider is built against the features currently under test.
  `crossplane-runtime/v2` moves from the `v2.5.0` tag to develop `8df966ac`, and
  `crossplane/apis/v2` from the `main` tip to develop `e33ad34be`. Both are pinned by
  commit via pseudo-version, because `develop` is a moving integration branch and tagging
  it would freeze a target that moves by design.

  This matters: the previously pinned `v2.5.0` runtime tag is on a different lineage from
  `develop`, 67 commits behind it and missing both the `APIRecorder` to
  `events.EventRecorder` migration and the `ExternalLister` interface. The `crossplane/apis/v2`
  pin was on the `main` tip, 28 commits behind `develop`, and so saw none of the resource
  discovery and import work. Neither repository uses a `master` branch; both track `main`,
  and both `develop` branches were already rebased onto it with nothing behind.

  The provider compiles, vets, tests and passes the full end to end suite against these
  versions. Regenerating the CRDs produces no schema change: only the `controller-gen`
  version annotation and some upstream wording differ, and all seven schema shapes are
  identical.
  - **Published the Crossplane 2.5.0 control plane the provider requires.** The package
    declares `crossplane.version: ">=v2.5.0"`, but upstream has no 2.5.0: the newest chart at
    `charts.crossplane.io/stable` is 2.4.2 and the newest stable image is `v2.2.2`. The
    documented install path, `helm repo add crossplane https://charts.crossplane.io/stable`,
    therefore produced a 2.4.2 control plane on which the provider install was rejected on the
    version constraint. Both v0.21.4 and v0.21.5 were published in that state.

    Two artifacts are now public, built from `rossigee/crossplane` `develop` at `e33ad34` with
    `buildVersion` pinned to `v2.5.0`:

    * `ghcr.io/rossigee/crossplane:v2.5.0`, a multi-arch manifest list over
      `linux/amd64`, `linux/arm64`, `linux/arm/v7` and `linux/ppc64le`.
    * `oci://ghcr.io/rossigee/charts/crossplane:2.5.0`, the upstream chart with
      `image.repository` set to `ghcr.io/rossigee/crossplane`. The stock chart defaults to
      `xpkg.crossplane.io/crossplane/crossplane`, so publishing it unmodified would have
      installed the absent upstream image, and once upstream does ship 2.5.0 it would have
      silently installed upstream Core rather than the build this provider was verified against.

    Both pull anonymously. `docs/installation.md` and `README.md` now install the provider from
    this registry, and note that any Crossplane >= 2.5.0 satisfies the floor.
  - A Crossplane v2.5.0 control plane can now be built and used, and the provider is
  verified working against it. Built from the `rossigee/crossplane` `develop` branch at
  `e33ad34` with nix. Note that the build requires refreshing the Go vendor hash first: the
  `root` hash pinned in `nix/vendor-hashes.nix` is stale on that branch, so the build fails
  with a fixed-output hash mismatch until `nix run .#tidy` is run. With `buildVersion` pinned
  to `v2.5.0` in `flake.nix` the resulting image reports `v2.5.0` and the chart is
  `crossplane-2.5.0`. Against that control plane the provider installs `HEALTHY` with the
  `>= v2.5.0` floor unchanged, and both the S3 and the admin API paths were exercised
  against a real MinIO: a `Bucket` reconciles to `Available` and appears in a server-side
  bucket listing, a `User` reconciles to `Available`, its generated credentials authenticate
  successfully, and both resources are removed from MinIO when deleted.
- **This release cannot be installed until a Crossplane v2.5.0 artifact is available.** The
  package requires `crossplane.version: ">=v2.5.0"`, and no such artifact is published.
  The latest upstream release is v2.4.2, `charts.crossplane.io/stable` tops out at v2.4.2
  across 161 chart versions, and there is no v2.5.0 or v2.5.0-rc.0 image for
  `crossplane/crossplane` on `docker.io`, `ghcr.io` or `xpkg.crossplane.io`. The
  `rossigee/crossplane` fork carries the upstream `v2.5.0-rc.0` tag but publishes no
  release and no image for it; its `ghcr.io/rossigee/crossplane` repository tops out at
  v2.4.x, with `latest` built on 2026-05-22. Installing this package against a released
  Crossplane fails with `incompatible Crossplane version: package is not compatible with
  Crossplane version`. The v2.5.0 requirement is retained deliberately; it must not be
  relaxed to v2.4.2.
- Added a path to run the end to end suite against a 2.5.0 control plane. The Go
  dependencies already come from the rossigee forks (`crossplane-runtime/v2 v2.5.0` and
  `crossplane/apis/v2 v2.5.0-rc.0`); only a runnable control plane was missing.
  `make crossplane-image` builds one from the fork's `develop` branch, and
  `make test-e2e CROSSPLANE_IMAGE_REPOSITORY=<repo> CROSSPLANE_IMAGE_TAG=v2.5.0` uses it.
  The `develop` flake pins the reported version through a `buildVersion` binding that
  defaults to `null` and then emits `v0.0.0-<lastModified>-<shortRev>`, so a build without
  it set would self-report `v0.0.0` and be rejected by this package's `>= v2.5.0`
  constraint. The target therefore sets it explicitly, the same way the fork's own CI
  does. Building it requires nix, which is not a dependency of this repository.
- The end to end suite now installs its control plane from the published OCI chart,
  `oci://ghcr.io/rossigee/charts/crossplane`, rather than through the build submodule's
  `controlplane.up`. That target only supports the classic `helm repo add` model, so it
  cannot consume an `oci://` reference, and it could not have worked here regardless: the
  provider requires Crossplane >= v2.5.0 and the newest chart in
  `charts.crossplane.io/stable` is 2.4.2, so its `helm install --version 2.5.0` fails
  outright. Serving the chart over a `file://` repo is not an alternative either, as helm
  has no `file` protocol handler for `helm repo add`. A new `crossplane-up` target creates
  the cluster, writes the suite's kubeconfig and installs the chart directly, and
  `test-e2e` depends on it in place of `controlplane.up`. This target also has to create
  the cluster itself rather than depending on `kind-kubeconfig`, which fails on a cluster
  that does not exist yet.
- The Crossplane image is side-loaded into the kind node rather than pulled by the node's
  own container runtime. The node fails to pull the multi-arch `v2.5.0` image from ghcr.io
  with `unable to fetch descriptor (sha256:d9ac45e64eb41...) which reports content size of
  zero: invalid argument`, leaving both `crossplane` and `crossplane-rbac-manager` in
  `ImagePullBackOff` until the wait expires. The image itself is sound: its index, per-arch
  manifests, config and layers were all checked, a host `docker pull` of the same reference
  succeeds and resolves the index to the amd64 manifest, and both pods become `Ready` as
  soon as that image is loaded into the node. The cause of the runtime's failure is not
  established, so the suite loads the image instead, which is how it already installs the
  provider image. This means `make test-e2e` now requires `docker` on `PATH`.
- Pinned kuttl to 0.27.0. The build submodule's `k8s_tools.mk` defaults to 0.12.1, which is
  what `test/e2e` was originally written against, and `test-e2e` did not depend on the
  kuttl target at all, so the suite would have run against whatever kuttl happened to be on
  `PATH`. The pin is set before that file is included, because it uses `?=` and so only
  applies its default when the version is unset.

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
