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

variable "primary" {
  type = object({
    project_id = string
    api_key    = string
    channel_id = string
  })
  sensitive = true
}

variable "secondary" {
  type = object({
    project_id = string
    api_key    = string
    check_id   = string
  })
  sensitive = true
}

provider "healthchecks" {
  alias    = "primary"
  base_url = var.base_url
  api_key  = var.primary.api_key
}

provider "healthchecks" {
  alias    = "secondary"
  base_url = var.base_url
  api_key  = var.secondary.api_key
}

data "healthchecks_integration" "alert" {
  provider   = healthchecks.primary
  project_id = nonsensitive(var.primary.project_id)
  id         = nonsensitive(var.primary.channel_id)
  type       = "webhook"
}

data "healthchecks_check" "existing" {
  provider   = healthchecks.secondary
  project_id = nonsensitive(var.secondary.project_id)
  id         = nonsensitive(var.secondary.check_id)
}

output "primary_channel_id" {
  value = data.healthchecks_integration.alert.id
}

output "secondary_check_status" {
  value = data.healthchecks_check.existing.status
}
