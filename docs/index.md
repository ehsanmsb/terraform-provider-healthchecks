# Healthchecks Provider

This provider manages Healthchecks projects, checks, webhook integrations, and project members.

Supported authentication today:

- Username/password session login against the Healthchecks web app.

Configuration:

```hcl
provider "healthchecks" {
  base_url = "https://healthchecks.example.com"
  username = var.healthchecks_username
  password = var.healthchecks_password
  timeout  = "30s"
}
```

See the `examples/` directory for end-to-end usage.

Project support includes sensitive computed attributes for:

- read-write API key
- read-only API key
- ping key

and boolean toggles for enabling or revoking those keys from Terraform.
