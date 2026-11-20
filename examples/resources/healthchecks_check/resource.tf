resource "healthchecks_project" "example" {
  name = "Checks Project"
}

resource "healthchecks_integration" "webhook" {
  project_id      = healthchecks_project.example.id
  project_api_key = healthchecks_project.example.api_key
  type            = "webhook"
  name            = "Checks Webhook"

  webhook = {
    method_down = "POST"
    url_down    = "https://example.com/down"
    body_down   = jsonencode({ state = "down" })
    method_up   = "POST"
    url_up      = "https://example.com/up"
    body_up     = jsonencode({ state = "up" })
  }
}

resource "healthchecks_check" "example" {
  project_id      = healthchecks_project.example.id
  project_api_key = healthchecks_project.example.api_key
  name            = "nightly-job"
  slug            = "nightly-job"
  timeout         = 3600
  grace           = 300
  tags            = ["batch", "nightly"]
  channels        = [healthchecks_integration.webhook.id]
}
