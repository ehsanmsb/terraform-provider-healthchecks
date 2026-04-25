resource "healthchecks_project" "example" {
  name = "Integrations Project"
}

resource "healthchecks_integration" "webhook" {
  project_id = healthchecks_project.example.id
  type       = "webhook"
  name       = "Primary Webhook"

  config = {
    method_down = "POST"
    url_down    = "https://example.com/down"
    body_down   = "{\"state\":\"down\"}"
    method_up   = "POST"
    url_up      = "https://example.com/up"
    body_up     = "{\"state\":\"up\"}"
  }
}

resource "healthchecks_integration" "email" {
  project_id = healthchecks_project.example.id
  type       = "email"

  config = {
    value = "alerts@example.com"
    down  = "true"
    up    = "false"
  }
}
