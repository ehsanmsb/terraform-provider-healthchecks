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
  default = "Terraform Project Keys Smoke Test"
}

variable "api_key_enabled" {
  type    = bool
  default = true
}

variable "read_only_api_key_enabled" {
  type    = bool
  default = true
}

variable "ping_key_enabled" {
  type    = bool
  default = true
}
