---
page_title: "Google Tag Manager Provider: スタートガイド"
description: |-
  Google Tag Manager Providerの認証と基本的なリソース管理を設定します。
---

# スタートガイド

## 前提条件

- Terraform 1.14以降
- Google Tag Manager APIを有効化したGoogle Cloudプロジェクト
- 対象GTMアカウントへのアクセス権を持つユーザーまたはサービスアカウント

サービスアカウントを使う場合は、そのメールアドレスをGTMのユーザーとして
追加し、操作に必要なAccountおよびContainer権限を付与します。

## 認証

ローカル開発ではApplication Default Credentialsを利用できます。

```shell
gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/tagmanager.readonly,https://www.googleapis.com/auth/tagmanager.edit.containers
```

CIではWorkload Identity Federationまたはサービスアカウント偽装を利用できます。
JSON資格情報を使う場合は、`GOOGLE_CREDENTIALS`へJSON本文を設定するか、
`GOOGLE_APPLICATION_CREDENTIALS`へファイルパスを設定します。

## Provider設定

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

## AccountとContainerの確認

```hcl
data "gtm_accounts" "available" {}

data "gtm_containers" "available" {
  parent = "accounts/123456"
}
```

複数形Data Sourceは既定で全ページを取得します。大規模なアカウントでは
`all_pages = false`と`page_token`を使ってページ単位で取得できます。

## Workspace内のリソース

v2のTag、Trigger、VariableなどはWorkspaceを親に持ちます。

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

APIの再帰的な`Parameter`や`Condition`は、`parameter_json`や
`filter_json`のようなJSON属性で設定します。`jsonencode`を使うとHCLの値から
安全にJSONを生成できます。

## Import

ResourceはAPI相対パスでimportします。

```shell
terraform import gtm_workspace.site accounts/123456/containers/789012/workspaces/3
```

Built-In Variableは末尾にtypeを追加します。

```shell
terraform import gtm_built_in_variable.page_url \
  accounts/123456/containers/789012/workspaces/3/built_in_variables/pageUrl
```

## 公開などの操作

Container Versionの公開、Workspaceの同期、Quick Preview、競合解決などは
Terraform Actionsとして実行します。Actionの結果はTerraform stateへ保存されない
ため、永続化された結果は対応するData Sourceから参照します。

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
