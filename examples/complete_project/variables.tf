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
  default = "Terraform Complete Project Example"
}

variable "email_integration_address" {
  type    = string
  default = "alerts@example.com"
}

variable "project_member_emails" {
  type = list(string)
  default = [
    "ali@snapp.cab",
    "ehsan@snapp.cab",
    "reza@snapp.cab",
  ]
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
