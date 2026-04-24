resource "healthchecks_project" "example" {
  name                      = "Terraform Managed Project"
  api_key_enabled           = true
  read_only_api_key_enabled = true
  ping_key_enabled          = false
}
