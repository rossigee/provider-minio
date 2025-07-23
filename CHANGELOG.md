# Changelog

## [v0.4.4-12-makelib] - 2025-07-23

### 🚀 Features
- **Build System Modernization**: Migrated from VSHN custom build system to standard Crossplane makelib
- **xpkg Package Generation**: Added proper xpkg package building for distribution via OCI registries
- **Multi-Architecture Support**: Enabled building for multiple platforms using makelib infrastructure
- **Standardized Make Targets**: Consistent build, test, and publishing commands across all Crossplane providers

### 🔧 Build System Changes
- Added official `crossplane/build` submodule for standardized build infrastructure
- Replaced VSHN-specific Makefile with makelib-compatible version
- Created proper Docker build configuration (`cluster/images/provider-minio/`)
- Removed obsolete VSHN build files (ci.mk, kind/kind.mk, Makefile.vars.mk)
- Updated build documentation and README for makelib patterns

### 📄 Documentation
- Updated README.md with makelib build system documentation
- Added build system overview and standardized make targets
- Removed outdated VSHN-specific build instructions
- Updated development and debugging workflows for makelib

### 🔗 Registry Integration  
- **Harbor Registry**: Successfully built and pushed to `harbor.golder.lan/library/provider-minio:v0.4.4-12-makelib`
- **xpkg Distribution**: Published xpkg package to `harbor.golder.lan/library/provider-minio:v0.4.4-12-makelib-xpkg`
- **Container Image**: 60.7MB Alpine 3.20-based image with proper security context

### ✅ Quality Assurance
- **All Tests Pass**: Verified 100% test suite compatibility with makelib conversion
- **ServiceAccount Functionality**: Confirmed all CRUD operations, validation, and edge cases work correctly
- **Build Reproducibility**: Standardized build process ensures consistent artifacts across environments

### Technical Details
- **Base Image**: Updated to Alpine 3.20 for latest security patches
- **Build System**: Crossplane makelib with build submodule integration
- **Package Format**: Modern xpkg format compatible with Crossplane >=v1.9.0
- **Registry**: Harbor OCI registry with proper authentication and CA trust

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