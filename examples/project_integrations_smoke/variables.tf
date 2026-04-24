variable "base_url" {
  type = string
}

variable "username" {
  type = string
}

variable "password" {
  type      = string
  sensitive = true
}

variable "project_name" {
  type    = string
  default = "Terraform Project Integrations Smoke Test"
}

variable "email_integration_address" {
  type    = string
  default = "alerts@example.com"
}
