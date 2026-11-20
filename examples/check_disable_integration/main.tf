resource "healthchecks_project" "example" {
  name = "Check Disable Integration Project"
}

resource "healthchecks_integration" "webhook" {
  project_id      = healthchecks_project.example.id
  project_api_key = healthchecks_project.example.api_key
  type            = "webhook"
  name            = "Disable Demo Webhook"

  webhook = {
    method_down = "POST"
    url_down    = "https://example.com/down"
    method_up   = "POST"
    url_up      = "https://example.com/up"
  }
}

resource "healthchecks_check" "enabled" {
  project_id      = healthchecks_project.example.id
  project_api_key = healthchecks_project.example.api_key
  name            = "job-with-webhook"
  channels        = [healthchecks_integration.webhook.id]
}

resource "healthchecks_check" "disabled" {
  project_id      = healthchecks_project.example.id
  project_api_key = healthchecks_project.example.api_key
  name            = "job-without-webhook"
  channels        = []
}
