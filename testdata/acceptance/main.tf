terraform {
  required_providers {
    healthchecks = {
      source = "ehsanmsb/healthchecks"
    }
  }
}

variable "base_url" {
  type = string
}

variable "username" {
  type      = string
  sensitive = true
}

variable "password" {
  type      = string
  sensitive = true
}

variable "member_email" {
  type = string
}

variable "project_name" {
  type = string
}

variable "keys_enabled" {
  type    = bool
  default = true
}

variable "project_key" {
  type      = string
  sensitive = true
  default   = null
}

variable "updated" {
  type    = bool
  default = false
}

variable "importing" {
  type    = bool
  default = false
}

provider "healthchecks" {
  base_url = var.base_url
  username = var.username
  password = var.password
}

provider "healthchecks" {
  alias    = "api"
  base_url = var.base_url
  api_key  = var.project_key
}

resource "healthchecks_project" "test" {
  name                      = var.updated ? "${var.project_name} updated" : var.project_name
  api_key_enabled           = var.keys_enabled
  read_only_api_key_enabled = var.keys_enabled
  ping_key_enabled          = var.keys_enabled
}

resource "healthchecks_integration" "webhook" {
  project_id      = healthchecks_project.test.id
  project_api_key = var.importing ? null : var.project_key
  type            = "webhook"
  name            = var.updated ? "Acceptance Webhook Updated" : "Acceptance Webhook"
  config = {
    method_down = "POST"
    url_down    = "https://example.test/down"
    method_up   = "POST"
    url_up      = "https://example.test/up"
  }
}

resource "healthchecks_integration" "email" {
  project_id      = healthchecks_project.test.id
  project_api_key = var.importing ? null : var.project_key
  type            = "email"
  config = {
    value = nonsensitive(var.username)
    down  = "true"
    up    = var.updated ? "false" : "true"
  }
}

data "healthchecks_project" "existing" {
  id = healthchecks_project.test.id
}

data "healthchecks_integration" "webhook" {
  provider   = healthchecks.api
  project_id = healthchecks_project.test.id
  type       = "webhook"
  id         = healthchecks_integration.webhook.id
}

data "healthchecks_integration" "email" {
  project_id      = healthchecks_project.test.id
  project_api_key = var.project_key
  type            = "email"
  id              = healthchecks_integration.email.id
}

resource "healthchecks_check" "test" {
  provider   = healthchecks.api
  project_id = healthchecks_project.test.id
  name       = var.updated ? "Acceptance Check Updated" : "Acceptance Check"
  slug       = "acceptance-check"
  channels   = var.updated ? [] : sort([data.healthchecks_integration.webhook.id, data.healthchecks_integration.email.id])
}

data "healthchecks_check" "test" {
  provider   = healthchecks.api
  project_id = healthchecks_project.test.id
  id         = healthchecks_check.test.id
}

resource "healthchecks_project_member" "test" {
  project_id = healthchecks_project.test.id
  email      = var.member_email
  role       = var.updated ? "w" : "m"
}

output "keys" {
  sensitive = true
  value = {
    read_write = healthchecks_project.test.api_key
    read_only  = healthchecks_project.test.read_only_api_key
    ping       = healthchecks_project.test.ping_key
  }
}

output "resources" {
  value = {
    project      = data.healthchecks_project.existing.id
    project_name = data.healthchecks_project.existing.name
    keys_enabled = alltrue([
      data.healthchecks_project.existing.api_key_enabled,
      data.healthchecks_project.existing.read_only_api_key_enabled,
      data.healthchecks_project.existing.ping_key_enabled,
    ])
    webhook_name = data.healthchecks_integration.webhook.name
    check        = healthchecks_check.test.id
    webhook      = healthchecks_integration.webhook.id
    email        = healthchecks_integration.email.id
    channels     = data.healthchecks_check.test.channels
  }
}
