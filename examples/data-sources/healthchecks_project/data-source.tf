data "healthchecks_project" "existing" {
  id = var.project_id
}

variable "project_id" {
  type = string
}
