resource "healthchecks_project" "example" {
  name = "Integrations Project"
}

resource "healthchecks_integration" "webhook" {
  project_id = healthchecks_project.example.id
  type       = "webhook"
  name       = "Primary Webhook"

  webhook = {
    method_down = "POST"
    url_down    = "https://example.com/down"
    body_down   = jsonencode({ state = "down" })
    headers_down = {
      X-Sample-Header = "$NAME has gone down"
      X-Env           = "production"
    }
    method_up = "POST"
    url_up    = "https://example.com/up"
    body_up   = jsonencode({ state = "up" })
    headers_up = {
      X-Sample-Header = "$NAME has recovered"
      X-Env           = "production"
    }
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
