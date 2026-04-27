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

## Provider Configuration

Example:

```hcl
provider "healthchecks" {
  base_url = var.healthchecks_base_url
  username = var.healthchecks_username
  password = var.healthchecks_password
  timeout  = var.healthchecks_timeout
}
```

You can pass credentials and optional settings through Terraform environment variables:

```bash
export TF_VAR_healthchecks_base_url="https://healthchecks.example.com"
export TF_VAR_healthchecks_username="your-email@example.com"
export TF_VAR_healthchecks_password="your-password"
export TF_VAR_healthchecks_timeout="30s"
```

For self-hosted instances with unusual certificates, you can also model and export:

```bash
export TF_VAR_healthchecks_insecure_skip_verify=true
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

This repository uses Semantic Versioning with `semantic-release` and Conventional Commits.

When a branch is merged into `main`, the release workflow analyzes the commits since the previous release, calculates the next version, creates the Git tag automatically, generates release notes, and runs GoReleaser to publish signed provider artifacts.

Release notes are grouped into reader-friendly sections:

- `Features`
- `Bug Fixes`
- `Performance`
- `Refactoring`

Low-signal commit types such as `docs:`, `test:`, `chore:`, `ci:`, and `build:` are hidden from the published release notes unless they also include a breaking change.

The repository includes:

- `.github/workflows/release.yml` for automatic releases on `main`
- `.releaserc.yml` for semantic-release configuration
- `.goreleaser.yml` for provider build artifacts
- `terraform-registry-manifest.json` for Terraform Registry metadata

### Commit Rules

Use standard Conventional Commit syntax:

- `feat: add project data source` → minor release
- `fix: handle project create redirect` → patch release
- `perf: reduce retry churn` → patch release
- `refactor: split webhook config conversion` → no version bump, but shown in release notes
- `feat!: change import ID format` → major release
- `BREAKING CHANGE: import format changed` in the commit footer → major release

Important:

- use `feat:` not `[feat]`
- use `fix:` not `[fix]`
- use `feat!:` or a `BREAKING CHANGE:` footer for major bumps
- commits like `docs:` and `chore:` do not trigger a release by default unless they also include a breaking change

Example breaking commit:

```text
feat!: rename project member role values

BREAKING CHANGE: project_member.role now uses long-form role names
```

This repository is configured to produce Terraform Registry-ready releases, including:

- zipped provider archives per OS/architecture
- `terraform-provider-healthchecks_<version>_SHA256SUMS`
- `terraform-provider-healthchecks_<version>_SHA256SUMS.sig`
- `terraform-provider-healthchecks_<version>_manifest.json`

### Terraform Registry Setup

Before publishing to the public Terraform Registry:

1. Generate a GPG keypair for signing provider releases.
2. Export the ASCII-armored private key and add it to GitHub Actions secrets as `GPG_PRIVATE_KEY`.
3. Add the private key passphrase to GitHub Actions secrets as `PASSPHRASE`.
4. Export the ASCII-armored public key and add it in Terraform Registry under `User Settings` or namespace `Signing Keys`.
5. Merge conventional commits into `main` to trigger the signed release workflow automatically.

HashiCorp requires provider release checksums to be signed, and the Terraform Registry validates releases against the uploaded public key.

Generate a Registry-compatible GPG key carefully:

- use RSA or DSA, not the default ECC key type
- keep the private key outside the repository
- avoid rotating the signing key casually once releases are published

Useful commands:

```bash
gpg --full-generate-key
gpg --armor --export-secret-keys "your-email@example.com"
gpg --armor --export "your-email@example.com"
```

GitHub Actions secrets required by `.github/workflows/release.yml`:

- `GPG_PRIVATE_KEY`
- `PASSPHRASE`

After the first signed GitHub release exists, publish the provider in Terraform Registry from the GitHub-backed namespace and let Registry create its webhook for future releases.

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
