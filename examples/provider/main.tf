terraform {
  required_providers {
    healthchecks = {
      source = "snapp/healthchecks"
    }
  }
}

provider "healthchecks" {
  base_url = var.base_url
  username = var.username
  password = var.password
}
