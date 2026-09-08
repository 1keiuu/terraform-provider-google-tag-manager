# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Added

- Complete Google Tag Manager API v1 and v2 provider surface.
- Application Default Credentials, validated service account and authorized
  user credential JSON, OAuth access tokens, and service account impersonation.
- Discovery coverage, HTTP contract, schema, and retry tests.

### Security

- Restricted authenticated API requests to the canonical Google Tag Manager
  endpoint.
- Marked structured JSON attributes as sensitive and hardened the signed
  release workflow and dependency checks.
