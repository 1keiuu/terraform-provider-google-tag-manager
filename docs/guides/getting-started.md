---
page_title: "Google Tag Manager Provider: Getting Started"
description: |-
  Configure authentication and manage Google Tag Manager resources with Terraform.
---

# Getting Started

## Prerequisites

- Terraform 1.14 or later
- A Google Cloud project with the Google Tag Manager API enabled
- An existing Google Tag Manager account
- A user or service account with access to the target account

When using a service account, add its email address as a Google Tag Manager user
and grant the Account and Container permissions required for the operations you
will perform.

## Initial account setup

A Google Cloud project and a Google Tag Manager account are separate resources.
Enabling the Google Tag Manager API in a Google Cloud project does not create a
Google Tag Manager account.

The Tag Manager API does not provide a method for creating accounts. Before
using this provider, [create an account and its first container in Google Tag
Manager](https://support.google.com/tagmanager/answer/14842164?hl=en). The
`gtm_account_settings` resource manages settings on that existing account, and
`gtm_container` creates additional containers under it.

## Authentication

For local development, use Application Default Credentials.

```shell
gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/tagmanager.readonly,https://www.googleapis.com/auth/tagmanager.edit.containers
```

In CI, use Workload Identity Federation or service account impersonation. JSON
configured through `credentials` or `GOOGLE_CREDENTIALS` must contain service
account or authorized user credentials. Credential files and Workload Identity
Federation should be configured through Application Default Credentials using
`GOOGLE_APPLICATION_CREDENTIALS`.

## Provider configuration

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
```

## Read accounts and containers

```hcl
data "gtm_accounts" "available" {}

data "gtm_containers" "available" {
  parent = "accounts/123456"
}
```

List data sources retrieve all pages by default. For large accounts, set
`all_pages = false` and use `page_token` to retrieve one page at a time.

## Create a container

Set `parent` to the API-relative path of an existing Google Tag Manager account
to create a container under that account.

```hcl
resource "gtm_container" "website" {
  parent        = "accounts/123456"
  name          = "www.example.com"
  usage_context = ["web"]
}
```

## Manage workspace resources

v2 tags, triggers, and variables use a workspace as their parent.

```hcl
resource "gtm_workspace" "site" {
  parent = "accounts/123456/containers/789012"
  name   = "Production changes"
}

resource "gtm_folder" "analytics" {
  parent = gtm_workspace.site.id
  name   = "Analytics"
}
```

Recursive `Parameter` and `Condition` values are configured through JSON
attributes such as `parameter_json` and `filter_json`. Use `jsonencode` to
generate JSON safely from HCL values.

Structured JSON attributes are marked sensitive to hide their values in
Terraform output and logs. Sensitivity does not encrypt state. When GTM
parameters or templates contain tokens, credentials, or custom code, use an
encrypted remote backend and restrict access to state.

## Import resources

Import resources by their API-relative path.

```shell
terraform import gtm_workspace.site accounts/123456/containers/789012/workspaces/3
```

Built-in variables append the variable type to the path.

```shell
terraform import gtm_built_in_variable.page_url \
  accounts/123456/containers/789012/workspaces/3/built_in_variables/pageUrl
```

## Invoke actions

Publishing container versions, synchronizing workspaces, running Quick Preview,
and resolving conflicts are Terraform actions. Action results are not stored in
Terraform state; read persistent results through the corresponding data source.

```hcl
action "gtm_publish_container_version" "production" {
  config {
    path = "accounts/123456/containers/789012/versions/4"
  }
}
```

```shell
terraform apply -invoke=action.gtm_publish_container_version.production
```
