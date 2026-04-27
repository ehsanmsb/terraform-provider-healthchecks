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

resource "healthchecks_project" "example" {
  name = var.project_name
  api_key_enabled           = var.api_key_enabled
  read_only_api_key_enabled = var.read_only_api_key_enabled
  ping_key_enabled          = var.ping_key_enabled
}

resource "healthchecks_integration" "webhook" {
  project_id = healthchecks_project.example.id
  type       = "webhook"
  name       = "project-webhook"

  webhook = {
    method_down = "POST"
    url_down    = "https://example.com/down"
    body_down   = jsonencode({ status = "down" })
    method_up   = "POST"
    url_up      = "https://example.com/up"
    body_up     = jsonencode({ status = "up" })
  }
}

resource "healthchecks_integration" "email" {
  project_id = healthchecks_project.example.id
  type       = "email"

  config = {
    value = var.email_integration_address
    down  = "true"
    up    = "false"
  }
}

locals {
  checks = {
    check_1 = {
      name = "nightly-job"
      slug = "nightly-job"
      tags = ["nightly", "batch"]
    }
    check_2 = {
      name = "hourly-sync"
      slug = "hourly-sync"
      tags = ["hourly", "sync"]
    }
    check_3 = {
      name = "report-export"
      slug = "report-export"
      tags = ["report", "export"]
    }
  }

  project_members = {
    for email in var.project_member_emails :
    replace(replace(email, "@", "_"), ".", "_") => {
      email = email
      role  = "w"
    }
  }
}

resource "healthchecks_check" "job" {
  for_each   = local.checks
  project_id = healthchecks_project.example.id
  name       = each.value.name
  slug       = each.value.slug
  timeout    = 3600
  grace      = 300
  tags       = each.value.tags
  channels = [
    healthchecks_integration.webhook.id,
    healthchecks_integration.email.id,
  ]
}

resource "healthchecks_project_member" "member" {
  for_each   = local.project_members
  project_id = healthchecks_project.example.id
  email      = each.value.email
  role       = each.value.role
}

output "project_id" {
  value = healthchecks_project.example.id
}

output "project_key_status" {
  value = {
    api_key_enabled           = healthchecks_project.example.api_key_enabled
    read_only_api_key_enabled = healthchecks_project.example.read_only_api_key_enabled
    ping_key_enabled          = healthchecks_project.example.ping_key_enabled
  }
}

output "check_uuids" {
  value = { for key, check in healthchecks_check.job : key => check.uuid }
}

output "webhook_channel_id" {
  value = healthchecks_integration.webhook.id
}

output "email_channel_id" {
  value = healthchecks_integration.email.id
}

output "api_key_enabled" {
  value = healthchecks_project.example.api_key_enabled
}

output "read_only_api_key_enabled" {
  value = healthchecks_project.example.read_only_api_key_enabled
}

output "ping_key_enabled" {
  value = healthchecks_project.example.ping_key_enabled
}
