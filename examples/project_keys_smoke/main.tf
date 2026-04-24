terraform {
  required_providers {
    healthchecks = {
      source = "ehsanmsb/healthchecks"
    }
  }
}

provider "healthchecks" {
  base_url = var.base_url
  username = var.username
  password = var.password
}

resource "healthchecks_project" "keys" {
  name                      = var.project_name
  api_key_enabled           = var.api_key_enabled
  read_only_api_key_enabled = var.read_only_api_key_enabled
  ping_key_enabled          = var.ping_key_enabled
}

output "project_id" {
  value = healthchecks_project.keys.id
}

output "api_key_enabled" {
  value = healthchecks_project.keys.api_key_enabled
}

output "read_only_api_key_enabled" {
  value = healthchecks_project.keys.read_only_api_key_enabled
}

output "ping_key_enabled" {
  value = healthchecks_project.keys.ping_key_enabled
}
