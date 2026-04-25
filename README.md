# terraform-provider-healthchecks

Terraform provider for [Healthchecks](https://github.com/healthchecks/healthchecks), implemented with the Terraform Plugin Framework.

## Current scope

- `healthchecks_project`
- `healthchecks_check`
- `healthchecks_integration` with `webhook` and `email` support
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

## Docs

Provider/resource documentation is checked with `tfplugindocs`.

Generate docs locally with:

```bash
go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.20.1
$(go env GOPATH)/bin/tfplugindocs generate
```

## CI

GitHub Actions runs the `CI` workflow on pull requests and pushes to `main`.

The workflow verifies:

- `gofmt`
- `go vet`
- `go test ./...`
- `go build ./...`
- `tfplugindocs` generation drift check

If you protect `main`, a good required status check is:

- `Quality`
- `Provider Docs`

## Releases

This repository uses Semantic Versioning and semantic commits.

- Use commit messages such as `feat: ...`, `fix: ...`, `docs: ...`, and `chore: ...`
- `release-please` watches `main` and prepares the next version from commit history
- pushing a tag like `v0.1.0` triggers GoReleaser to build release artifacts and publish a GitHub release

The repository includes:

- `.github/workflows/release-please.yml` for semantic version automation
- `.github/workflows/release.yml` for tagged releases
- `.goreleaser.yml` for provider build artifacts
- `terraform-registry-manifest.json` for Terraform Registry metadata

Note: publishing to the public Terraform Registry also requires signed release checksums and an uploaded public GPG key in the Registry settings.

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

- Expand `healthchecks_integration` beyond `webhook` to additional Healthchecks integration types.
- Add direct API key authentication to provider configuration.
- Add richer acceptance tests with real CRUD coverage.
- Add first-class project data sources.
- Improve import behavior so imported projects/checks do not need API-key regeneration.
- Add acceptance coverage for project key enable/disable behavior.

## Roadmap

See [docs/roadmap.md](docs/roadmap.md) for a more structured list of candidate enhancements that can be turned into GitHub issues.
