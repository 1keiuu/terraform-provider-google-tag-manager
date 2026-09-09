---
page_title: "Google Tag Manager Provider: スタートガイド"
description: |-
  Google Tag Manager Providerの認証と基本的なリソース管理を設定します。
---

# スタートガイド

## 前提条件

- Terraform 1.14以降
- Google Tag Manager APIを有効化したGoogle Cloudプロジェクト
- 作成済みのGTMアカウント
- 対象GTMアカウントへのアクセス権を持つユーザーまたはサービスアカウント

サービスアカウントを使う場合は、そのメールアドレスをGTMのユーザーとして
追加し、操作に必要なAccountおよびContainer権限を付与します。

## GTMアカウントの初期作成

Google CloudプロジェクトとGTMアカウントは別のリソースです。Google Cloud
プロジェクトでGoogle Tag Manager APIを有効化しても、GTMアカウントは作成
されません。

Tag Manager APIにはGTMアカウントを作成するメソッドが存在しない為、このProviderを
使う前に、[Google Tag Managerでアカウントと最初のContainerを作成](https://support.google.com/tagmanager/answer/14842164?hl=ja)
してください。`gtm_account_settings`はその既存アカウントの設定を管理し、
`gtm_container`は既存アカウントの配下に追加のContainerを作成します。

## 認証

ローカル開発ではApplication Default Credentialsを利用できます。

```shell
gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/tagmanager.readonly,https://www.googleapis.com/auth/tagmanager.edit.containers
```

CIではWorkload Identity Federationまたはサービスアカウント偽装を利用できます。
`credentials`または`GOOGLE_CREDENTIALS`へ直接設定できるJSONは、サービス
アカウントまたは認可済みユーザーの資格情報に限られます。資格情報ファイルや
Workload Identity Federationは、`GOOGLE_APPLICATION_CREDENTIALS`を使った
Application Default Credentialsとして設定します。

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

## Containerの作成

GTMアカウントのAPI相対パスを`parent`に指定すると、既存アカウントの配下に
Containerを作成できます。

```hcl
resource "gtm_container" "website" {
  parent        = "accounts/123456"
  name          = "www.example.com"
  usage_context = ["web"]
}
```

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

構造化されたJSON属性は、Terraformの出力やログで値を隠すため機密属性として
扱われます。ただし、機密属性の指定はstateを暗号化しません。GTMのParameterや
Templateへトークン、資格情報、カスタムコードを設定する場合は、暗号化された
remote backendを使い、stateへのアクセスを必要最小限に制限してください。

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
