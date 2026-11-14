# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Fixed

- Keep the prior state when an update fails, instead of saving the planned values with `id`, `path` and the other
  computed attributes null, which made every later refresh fail with `path "" does not provide path`.
- Preserve configured custom template data when GTM removes a leading UTF-8 BOM, rewrites JSON escaping, adds blank lines around
  JavaScript sections, or removes the terms-of-service header, preventing inconsistent apply results
  and recurring template data differences after refresh.

### Added

- Complete Google Tag Manager API v1 and v2 provider surface.
- Application Default Credentials, validated service account and authorized
  user credential JSON, OAuth access tokens, and service account impersonation.
- Discovery coverage, HTTP contract, schema, and retry tests.

### Security

- Use Go 1.26.11 / 1.27.4 to address GO-2026-6859, update golang.org/x/net
  to v0.62.0 for GO-2026-6860 and GO-2026-6866, and update golang.org/x/crypto
  to v0.59.0 to exclude the unmaintained OpenPGP packages covered by GO-2026-5932.
- Update golang.org/x/net to v0.60.0 and use Go 1.26.10 / 1.27.3 for builds
  and CI to address HTTP/2 and certificate verification vulnerabilities.

- Restricted authenticated API requests to the canonical Google Tag Manager
  endpoint.
- Marked structured JSON attributes as sensitive and hardened the signed
  release workflow and dependency checks.
