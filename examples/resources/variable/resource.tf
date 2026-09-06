resource "gtm_variable" "measurement_id" {
  parent = "accounts/123456/containers/789012/workspaces/3"
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
