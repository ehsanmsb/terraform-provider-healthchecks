# Resource: healthchecks_check

Manages a Healthchecks check through `/api/v3/checks/`.

## Example

```hcl
resource "healthchecks_check" "job" {
  project_id = healthchecks_project.example.id
  name       = "nightly-job"
  timeout    = 3600
  grace      = 300
}
```

## Schema

- `project_id` (String, Required)
- `name` (String, Required)
- `slug` (String, Optional)
- `tags` (List of String, Optional/Computed)
- `desc` (String, Optional)
- `timeout` (Number, Optional)
- `grace` (Number, Optional)
- `schedule` (String, Optional)
- `tz` (String, Optional)
- `channels` (List of String, Optional/Computed)
- `id` (String, Computed)
- `uuid` (String, Computed)
- `ping_url` (String, Computed)
- `status` (String, Computed)
