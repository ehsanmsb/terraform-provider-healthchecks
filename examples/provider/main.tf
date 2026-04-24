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
