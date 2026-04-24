variable "base_url" {
  type    = string
  default = "http://localhost:8000"
}

variable "username" {
  type      = string
  sensitive = true
}

variable "password" {
  type      = string
  sensitive = true
}
