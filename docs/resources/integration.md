# Resource: healthchecks_integration

Manages a Healthchecks integration. The current implementation supports `type = "webhook"` and `type = "email"` through the Healthchecks web UI endpoints.

## Example

```hcl
resource "healthchecks_integration" "webhook" {
  project_id = healthchecks_project.example.id
  type       = "webhook"
  name       = "Primary Webhook"

  config = {
    method_down = "POST"
    url_down    = "https://example.com/down"
    method_up   = "POST"
    url_up      = "https://example.com/up"
  }
}
```

```hcl
resource "healthchecks_integration" "email" {
  project_id = healthchecks_project.example.id
  type       = "email"

  config = {
    value = "alerts@example.com"
    down  = "true"
    up    = "false"
  }
}
```

Attach the created integration to a check:

```hcl
resource "healthchecks_check" "job" {
  project_id = healthchecks_project.example.id
  name       = "nightly-job"
  channels   = [healthchecks_integration.webhook.id]
}
```

## Schema

- `project_id` (String, Required)
- `type` (String, Required)
- `name` (String, Optional)
- `config` (Map of String, Required)
- `id` (String, Computed)

## Notes

- Supported types today: `webhook`, `email`.
- Check-level integration enable/disable is controlled through `healthchecks_check.channels`.
- Removing a channel ID from `channels`, or setting `channels = []`, disables that integration for the check.
- For `type = "email"`, `config.value` is the destination email address and `config.up` / `config.down` control which state changes send alerts.
