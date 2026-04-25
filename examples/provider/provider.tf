provider "healthchecks" {
  base_url = "https://healthchecks.example.com"
  username = var.healthchecks_username
  password = var.healthchecks_password
  timeout  = "30s"
}

variable "healthchecks_username" {
  type = string
}

variable "healthchecks_password" {
  type      = string
  sensitive = true
}
