# Terraform Provider for Google Tag Manager

The Google Tag Manager provider manages GTM accounts, containers, workspaces,
tags, triggers, variables, templates, server-side entities, versions, and user
permissions with Terraform.

The provider implements every method in the bundled Google Tag Manager API v1
and v2 Discovery documents:

- 49 API v1 methods
- 106 API v2 methods
- 24 managed resource types
- 58 data source types
- 30 Terraform actions

Current API v2 types use the `gtm_` prefix. API v1 types use `gtm_v1_`.

## Requirements

- Terraform 1.14 or later
- Google Tag Manager API enabled in a Google Cloud project
- An existing Google Tag Manager account
- An OAuth user or service account with access to that account

## Initial account setup

A Google Cloud project and a Google Tag Manager account are separate. Enabling
the Tag Manager API in a Google Cloud project allows applications to call the
API, but it does not create a Google Tag Manager account.

The Tag Manager API does not provide a method for creating accounts. Before
using this provider, [create an account and its first container in Google Tag
Manager](https://support.google.com/tagmanager/answer/14842164). The
`gtm_account_settings` resource can then manage settings on that existing
account, and the `gtm_container` resource can create additional containers
under it.

## Usage

```hcl
terraform {
  required_version = ">= 1.14.0"

  required_providers {
    gtm = {
      source  = "1keiuu/google-tag-manager"
      version = "~> 0.1"
    }
  }
}

provider "gtm" {}

data "gtm_accounts" "available" {}
```

After the account exists, create an additional web container by passing its API
relative path to `parent`:

```hcl
resource "gtm_container" "website" {
  parent        = "accounts/123456"
  name          = "www.example.com"
  usage_context = ["web"]
}
```

Application Default Credentials are used when the provider has no explicit
authentication settings. The `credentials` attribute accepts inline service
account or authorized user JSON. Credential files and Workload Identity
Federation should be configured through Application Default Credentials using
`GOOGLE_APPLICATION_CREDENTIALS`. OAuth access tokens and service account
impersonation are also supported.

```hcl
resource "gtm_workspace" "site" {
  parent      = "accounts/123456/containers/789012"
  name        = "Managed by Terraform"
  description = "Production website configuration"
}

resource "gtm_variable" "measurement_id" {
  parent = gtm_workspace.site.id
  name   = "GA4 Measurement ID"
  type   = "c"

  parameter_json = jsonencode([
    {
      key   = "value"
      type  = "template"
      value = "G-XXXXXXXXXX"
    }
  ])
}
```

Operations such as publishing, synchronizing, previewing, and restoring are
Terraform actions:

```hcl
action "gtm_publish_container_version" "production" {
  config {
    path = "accounts/123456/containers/789012/versions/4"
  }
}
```

Invoke the action with Terraform:

```shell
terraform apply -invoke=action.gtm_publish_container_version.production
```

## API model

Scalar values, enums, and primitive collections are represented as typed HCL
attributes. Recursive or polymorphic GTM values use canonical JSON attributes
ending in `_json`. This preserves the full GTM `Parameter`, `Condition`, and
`Entity` structures without limiting nesting depth.

Resource IDs are canonical API-relative paths such as
`accounts/123456/containers/789012/workspaces/3/tags/7`. They can be passed
directly to a child resource's `parent` attribute or used for import.

## State security

Structured `_json` attributes are marked sensitive to reduce accidental
exposure in Terraform output and logs. Terraform sensitivity is display
metadata and does not encrypt state. GTM parameters and templates can contain
credentials, tokens, or custom code, so use an encrypted remote backend with
strict access controls and avoid placing secrets in GTM configuration when a
secret-management integration is available.

## Development

```shell
make test
make generate
make build
```

The standard test suite uses local API fixtures and does not require Google
credentials. Validate release candidates against a dedicated GTM account before
publishing them.

See [CONTRIBUTING.md](CONTRIBUTING.md) for development and testing details.
See the [Getting Started guide](docs/guides/getting-started.md) for the
authentication and resource setup steps.

## License

Mozilla Public License 2.0. See [LICENSE](LICENSE).
