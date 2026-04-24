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

locals {
  checks = {
    local_smoke_1 = {
      name = "local-smoke-1"
      slug = "local-smoke-1"
    }
    local_smoke_2 = {
      name = "local-smoke-2"
      slug = "local-smoke-2"
    }
    local_smoke_3 = {
      name = "local-smoke-3"
      slug = "local-smoke-3"
    }
  }

  project_members = {
    ali = {
      email = "test1@local.user"
      role  = "w"
    }
    ehsan = {
      email = "test2@local.user"
      role  = "w"
    }
    reza = {
      email = "test3@local.user"
      role  = "w"
    }
  }
}

resource "healthchecks_check" "job" {
  for_each   = local.checks
  project_id = healthchecks_project.smoke.id
  name       = each.value.name
  slug       = each.value.slug
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
  for_each   = local.project_members
  project_id = healthchecks_project.smoke.id
  email      = each.value.email
  role       = each.value.role
}

output "project_id" {
  value = healthchecks_project.smoke.id
}

output "check_uuids" {
  value = { for key, check in healthchecks_check.job : key => check.uuid }
}
