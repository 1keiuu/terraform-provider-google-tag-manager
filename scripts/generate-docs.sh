#!/bin/sh

set -eu

temporary_directory="$(mktemp -d)"
trap 'rm -rf "$temporary_directory"' EXIT

if [ -d docs/guides ]; then
  cp -R docs/guides "$temporary_directory/guides"
fi

go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.24.0 \
  generate \
  --provider-name gtm \
  --rendered-provider-name google-tag-manager

if [ -d "$temporary_directory/guides" ]; then
  mkdir -p docs
  cp -R "$temporary_directory/guides" docs/guides
fi
