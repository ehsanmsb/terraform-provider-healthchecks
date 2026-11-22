# Local Testing

This provider can be smoke-tested against a self-hosted Healthchecks instance running on `http://localhost:8000`.

## 1. Start Healthchecks with Docker

From the upstream checkout:

```bash
cp /tmp/healthchecks-upstream/docker/.env.example /tmp/healthchecks-upstream/docker/.env
```

Set at least:

```bash
ALLOWED_HOSTS=localhost
SITE_ROOT=http://localhost:8000
SECRET_KEY=local-dev-secret-key-change-me
DB_PASSWORD=postgres-local-pass
DEFAULT_FROM_EMAIL=hc-local@example.test
EMAIL_HOST=
```

Then start it:

```bash
docker compose -f /tmp/healthchecks-upstream/docker/docker-compose.yml \
  --env-file /tmp/healthchecks-upstream/docker/.env \
  up --build -d
```

Create a login user. For acceptance tests, create a separate member account
with the same command using `--email member@example.test`.

```bash
docker compose -f /tmp/healthchecks-upstream/docker/docker-compose.yml \
  --env-file /tmp/healthchecks-upstream/docker/.env \
  exec web /opt/healthchecks/manage.py createsuperuser --email admin@example.test
```

## 2. Run the Acceptance Suite

The acceptance suite builds the provider and runs Terraform against a disposable
self-hosted Healthchecks instance. Use two existing local accounts, one with a
password and permission to create projects and a separate account to add as a
project member. Use the owner account's exact email address as the username so
Healthchecks can verify its email integration without SMTP. Use the published
Docker host port for `HEALTHCHECKS_BASE_URL`.

From the provider repository root, with Go and Terraform on your PATH:

```bash
export HEALTHCHECKS_BASE_URL=http://localhost:8000
export HEALTHCHECKS_USERNAME=admin@example.test
export HEALTHCHECKS_TEST_MEMBER_EMAIL=member@example.test
IFS= read -r -s HEALTHCHECKS_PASSWORD
export HEALTHCHECKS_PASSWORD
TF_ACC=1 go test -v -count=1 -run '^TestAccProvider$' .
unset HEALTHCHECKS_PASSWORD
```

Enter the local account's password when the `read` command waits for input.
For consecutive runs, use a fresh disposable owner account to stay within the
self-hosted instance's per-account password login limit.

The test enables, disables and re-enables all three project key types. It
creates, refreshes, updates, imports and deletes a project, a check, webhook, email and
ntfy integrations, and a project member whose email is supplied in uppercase.
It verifies the channel changes through the check data source and checks for drift after update and import. Terraform state and the CLI dev override live in temporary directories.
The test attempts to destroy its project on failure. If cleanup fails, remove
the test project named `Terraform Acceptance …` through the local UI.

## 3. Build the Provider for Manual Smoke Tests

From this repository:

```bash
mkdir -p .dist
go build -o .dist/terraform-provider-healthchecks .
```

## 4. Use Terraform Dev Overrides

Create a local CLI config file:

```hcl
provider_installation {
  dev_overrides {
    "ehsanmsb/healthchecks" = "/ABSOLUTE/PATH/TO/terraform-provider-healthchecks/.dist"
  }
  direct {}
}
```

## 5. Run the Smoke Test

Use the example in [examples/local_smoke](../examples/local_smoke/main.tf):

```bash
cd examples/local_smoke
TF_VAR_username=admin@example.test \
TF_VAR_password=adminpass123 \
TF_CLI_CONFIG_FILE=/path/to/dev.tfrc \
terraform apply
```

When using Terraform `dev_overrides`, you can skip `terraform init`.

This exercises:

- `healthchecks_project`
- `healthchecks_check`
- `healthchecks_integration` with `webhook`
- `healthchecks_project_member`

## 6. Focused Project Key Test

Use [examples/project_keys_smoke](../examples/project_keys_smoke/main.tf) to test just project key creation and toggling:

```bash
cd examples/project_keys_smoke
TF_VAR_username=admin@example.test \
TF_VAR_password=adminpass123 \
TF_CLI_CONFIG_FILE=/path/to/dev.tfrc \
terraform apply
```

This focused example exercises:

- `api_key_enabled`
- `read_only_api_key_enabled`
- `ping_key_enabled`

## 7. Focused Project Integration Test

Use [examples/project_integrations_smoke](../examples/project_integrations_smoke/main.tf) to test a project with webhook and email integrations and a check wired to both:

```bash
cd examples/project_integrations_smoke
TF_VAR_username=admin@example.test \
TF_VAR_password=adminpass123 \
TF_CLI_CONFIG_FILE=/path/to/dev.tfrc \
terraform apply -parallelism=1
```

This focused example exercises:

- `healthchecks_integration` with `type = "webhook"`
- `healthchecks_integration` with `type = "email"`
- `healthchecks_check.channels` attachment to both integrations

## 8. Complete Project Example

Use [examples/complete_project](../examples/complete_project/main.tf) for a fuller example that creates:

- one project
- three checks
- two integrations (`webhook` and `email`)
- three project members

Run it like this:

```bash
cd examples/complete_project
TF_VAR_username=admin@example.test \
TF_VAR_password=adminpass123 \
TF_CLI_CONFIG_FILE=/path/to/dev.tfrc \
terraform apply -parallelism=1
```

## 9. Look Up Resources Across Two Projects

The [multi-project example](../examples/multi_project_api/main.tf) uses two
provider aliases with a separate read-write API key for each project. It reads
a webhook channel from one project and a check from the other.

Set `TF_VAR_base_url` to the local instance and provide `TF_VAR_primary` and
`TF_VAR_secondary` as JSON objects. The first object takes `project_id`,
`api_key`, and `channel_id`. The second takes `project_id`, `api_key`,
and `check_id`. Run `terraform plan` from the example directory.

To run against a development build, use the CLI dev override from step 4.
For a Registry build, run `terraform init` before planning.

## 10. Inspect Provider Debug Logs

Set `TF_LOG_PROVIDER=DEBUG` when troubleshooting. For example, run
`TF_LOG_PROVIDER=DEBUG terraform plan` from a fixture directory with the
credentials set in the environment. Provider debug messages describe login,
key toggles and integration form outcomes. Keep Terraform state and log files
private when collecting diagnostics.
