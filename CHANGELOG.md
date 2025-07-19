# Changelog

## [v0.2.0-serviceaccounts] - 2025-07-19

### Added
- ServiceAccount resource support for managing MinIO service accounts
- Full CRUD operations for ServiceAccount lifecycle management
- Connection secret generation with accessKey/secretKey credentials
- Support for policy attachment, expiry dates, and descriptions
- Webhook validation for ServiceAccount resources

### Changed
- Upgraded madmin-go from v3 to v4 for latest API compatibility
- Updated all dependencies to latest versions

### Technical Details
- New CRD: `serviceaccounts.minio.crossplane.io`
- Controller implementation in `operator/serviceaccount/`
- Example manifests in `examples/` directory

## Previous Releases
See git history for earlier versions.