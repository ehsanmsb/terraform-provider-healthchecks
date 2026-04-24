# Resource: healthchecks_project

Manages a Healthchecks project.

## Example

```hcl
resource "healthchecks_project" "example" {
  name = "Terraform Managed Project"
}
```

## Schema

- `name` (String, Required)
- `api_key_enabled` (Bool, Optional/Computed)
- `id` (String, Computed)
- `api_key` (String, Sensitive, Computed)
