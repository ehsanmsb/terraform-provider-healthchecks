data "healthchecks_check" "existing" {
  project_id = var.healthchecks_project_id
  id         = var.healthchecks_check_uuid
}

variable "healthchecks_project_id" {
  type = string
}

variable "healthchecks_check_uuid" {
  type = string
}
