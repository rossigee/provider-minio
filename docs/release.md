# Release Guide

## Versioning

`provider-minio` follows SemVer with `v` prefix, as defined in `VERSION` (`v0.21.3`). Pre-releases may use `-rcN` suffix (e.g. `v0.20.0-rc1`) — pattern from legacy `docs/modules/ROOT/pages/how-tos/create-releases.adoc:15`.

## How to Release

Releasing requires pushing an exact SemVer git tag (`.github/workflows/release.yml` handles publishing):

```bash
# Ensure main is clean and CI green
git checkout master
git pull
make generate && make lint && make test

# Tag
git tag v0.21.3
git push origin v0.21.3
```

CI `release.yml` then:

* Validates exact SemVer and that the tag points at `origin/master`
* Builds `linux_amd64` and `linux_arm64` xpkg files
* Publishes the version tag and aliases `latest`
* Verifies equal digests and both Linux architectures
* Creates GitHub Release with auto-generated release notes

> `publish.artifacts` is gated to `main|master|release-*` branches (`Makefile:41`). Tag-based releases run via `release.yml`, not `ci.yml` (`ci.yml:189` `build-validation` only).

## Changelog

Auto-created from merged PRs. PRs must have:

* `area:operator` plus one of `bug`, `enhancement`, `documentation`, `change`, `breaking`, `dependency` (legacy rule from `create-releases.adoc:27`).

Example tags:

* `v0.1.2`, `v1.4.0`, `v2.0.0-rc1`

## Pre-Release Checklist

* [ ] `make generate` — no diff (`make reviewable`)
* [ ] `make test` / `make lint` green
* [ ] `docs/API.md` and `README.md` updated (see `docs/standards/README-STANDARD.md` in `rossigee/crossplane-providers`)
* [ ] `VERSION` bumped (and `README.md:15` container tag)
* [ ] `CHANGELOG` or release notes drafted if breaking
* [ ] Examples in `examples/v2/` and `samples/` consistent (run `go generate` to refresh `samples/`)

## Crossplane Version

`Makefile:69` pins `CROSSPLANE_VERSION = 2.5.0`; `crossplane.yaml:66` requires `>=v2.5.0`. Managed resources are `minio.m.crossplane.io/v1beta1` namespaced since `v0.16.5+` (`README.md:150`).

## Registries

* Primary: `ghcr.io/rossigee/provider-minio` (always)
* Harbor removed (`Makefile:54` comment)
* Upbound optional via `ENABLE_UPBOUND_PUBLISH=true` and `XPKG_REG_ORGS=xpkg.upbound.io/crossplane-contrib`

See `.github/workflows/release.yml` and `.github/workflows/ci.yml` for full pipeline.
