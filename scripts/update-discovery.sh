#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

for version in v1 v2; do
  curl -fsSL "https://tagmanager.googleapis.com/\$discovery/rest?version=${version}" \
    -o "${repo_root}/internal/discovery/${version}.json"
done

(
  cd "${repo_root}"
  shasum -a 256 internal/discovery/v1.json internal/discovery/v2.json > internal/discovery/SHA256SUMS
  go test ./internal/provider -run TestEveryDiscoveryMethodIsCovered
)
