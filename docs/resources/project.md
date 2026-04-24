# Resource: healthchecks_project

Manages a Healthchecks project.

## Example

```hcl
resource "healthchecks_project" "example" {
  name                      = "Terraform Managed Project"
  api_key_enabled           = true
  read_only_api_key_enabled = true
  ping_key_enabled          = true
}
```

## Schema

- `name` (String, Required)
- `api_key_enabled` (Bool, Optional/Computed)
- `read_only_api_key_enabled` (Bool, Optional/Computed)
- `ping_key_enabled` (Bool, Optional/Computed)
- `id` (String, Computed)
- `api_key` (String, Sensitive, Computed)
- `read_only_api_key` (String, Sensitive, Computed)
- `ping_key` (String, Sensitive, Computed)

## Notes

- `api_key` is the read-write project API key used by the Management API.
- `read_only_api_key` is intended for read-only API access, such as dashboards.
- `ping_key` is the project secret used for slug-based ping URLs.
- These values are marked sensitive and are stored in Terraform state. Avoid echoing them through normal outputs unless you explicitly need that behavior.
