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
  name = "Terraform Local Smoke Test"
}

resource "healthchecks_check" "job" {
  project_id = healthchecks_project.smoke.id
  name       = "local-smoke"
  slug       = "local-smoke"
  timeout    = 3600
  grace      = 300
  tags       = ["local", "smoke"]
}

resource "healthchecks_integration" "webhook" {
  project_id = healthchecks_project.smoke.id
  type       = "webhook"
  name       = "webhook-smoke"

  config = {
    method_down = "POST"
    url_down    = "https://example.com/down"
    body_down   = "{\"status\":\"down\"}"
    method_up   = "POST"
    url_up      = "https://example.com/up"
    body_up     = "{\"status\":\"up\"}"
  }
}

resource "healthchecks_project_member" "member" {
  project_id = healthchecks_project.smoke.id
  email      = "viewer@example.test"
  role       = "r"
}

output "project_id" {
  value = healthchecks_project.smoke.id
}

output "check_uuid" {
  value = healthchecks_check.job.uuid
}
