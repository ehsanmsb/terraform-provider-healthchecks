# Resource: healthchecks_integration

Manages a Healthchecks integration. The initial implementation supports `type = "webhook"` through the Healthchecks web UI endpoints.

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

## Schema

- `project_id` (String, Required)
- `type` (String, Required)
- `name` (String, Optional)
- `config` (Map of String, Required)
- `id` (String, Computed)
