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
