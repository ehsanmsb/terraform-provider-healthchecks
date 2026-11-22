data "healthchecks_integration" "alert" {
  project_id = var.project_id
  id         = var.channel_id
  type       = "webhook"
}

variable "project_id" {
  type = string
}

variable "channel_id" {
  type = string
}
