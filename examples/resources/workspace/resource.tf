resource "gtm_workspace" "example" {
  parent      = "accounts/123456/containers/789012"
  name        = "Terraform workspace"
  description = "Managed configuration"
}
