# terraform-provider-healthchecks

Terraform provider for [Healthchecks](https://github.com/healthchecks/healthchecks), implemented with the Terraform Plugin Framework.

## Current scope

- `healthchecks_project`
- `healthchecks_check`
- `healthchecks_integration` with initial `webhook` support
- `healthchecks_project_member`

Project resources can also manage project-level secrets and access toggles:

- read-write API key
- read-only API key
- ping key

This implementation targets upstream Healthchecks commit `a5708269858d6fcd7b9934161aaceba13250d176` and derives its web/API flows from the Django source, views, forms, templates, and tests.

## Build

```bash
go mod tidy
go test ./...
go build ./...
```

## Acceptance tests

Set:

- `HEALTHCHECKS_BASE_URL`
- `HEALTHCHECKS_USERNAME`
- `HEALTHCHECKS_PASSWORD`

Then run:

```bash
go test -v ./...
```

## Notes

- Provider credentials are sensitive.
- Checks are managed through `/api/v3/checks/`.
- Projects, webhook integrations, and project members currently use authenticated web form endpoints.
- Because Healthchecks stores hashed project API keys server-side, the provider may mint a new RW project API key when it cannot recover plaintext from state or the current response.
- Project key values are marked sensitive and should generally not be re-exposed through Terraform outputs.

## TODO

- Add direct API key authentication to provider configuration.
- Add richer acceptance tests with real CRUD coverage.
- Add more integration kinds beyond webhooks.
- Add first-class project data sources.
- Improve import behavior so imported projects/checks do not need API-key regeneration.
- Add acceptance coverage for project key enable/disable behavior.
