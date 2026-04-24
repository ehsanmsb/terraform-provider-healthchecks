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

resource "healthchecks_project" "smoke" {
  name = var.project_name
}

resource "healthchecks_integration" "webhook" {
  project_id = healthchecks_project.smoke.id
  type       = "webhook"
  name       = "smoke-webhook"

  config = {
    method_down = "POST"
    url_down    = "https://example.com/down"
    body_down   = "{\"state\":\"down\"}"
    method_up   = "POST"
    url_up      = "https://example.com/up"
    body_up     = "{\"state\":\"up\"}"
  }
}

resource "healthchecks_integration" "email" {
  project_id = healthchecks_project.smoke.id
  type       = "email"

  config = {
    value = var.email_integration_address
    down  = "true"
    up    = "false"
  }
}

resource "healthchecks_check" "with_both" {
  project_id = healthchecks_project.smoke.id
  name       = "project-integrations-smoke"
  slug       = "project-integrations-smoke"
  timeout    = 3600
  grace      = 300
  channels = [
    healthchecks_integration.webhook.id,
    healthchecks_integration.email.id,
  ]
}

output "project_id" {
  value = healthchecks_project.smoke.id
}

output "webhook_channel_id" {
  value = healthchecks_integration.webhook.id
}

output "email_channel_id" {
  value = healthchecks_integration.email.id
}

output "check_uuid" {
  value = healthchecks_check.with_both.uuid
}
