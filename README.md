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

This repository uses Semantic Versioning and semantic commits.

- Use commit messages such as `feat: ...`, `fix: ...`, `docs: ...`, and `chore: ...`
- `release-please` watches `main` and prepares the next version from commit history
- pushing a tag like `v0.1.0` triggers GoReleaser to build release artifacts and publish a GitHub release

The repository includes:

- `.github/workflows/release-please.yml` for semantic version automation
- `.github/workflows/release.yml` for tagged releases
- `.goreleaser.yml` for provider build artifacts
- `terraform-registry-manifest.json` for Terraform Registry metadata

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
5. Push a SemVer tag such as `v0.2.0` to trigger the signed release workflow.

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
