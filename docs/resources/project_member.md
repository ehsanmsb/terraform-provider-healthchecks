# Resource: healthchecks_project_member

Manages project team membership through the project settings page.

## Example

```hcl
resource "healthchecks_project_member" "manager" {
  project_id = healthchecks_project.example.id
  email      = "teammate@example.com"
  role       = "m"
}
```

## Schema

- `project_id` (String, Required)
- `email` (String, Required)
- `role` (String, Required)
- `id` (String, Computed)
