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

Create a login user:

```bash
docker compose -f /tmp/healthchecks-upstream/docker/docker-compose.yml \
  --env-file /tmp/healthchecks-upstream/docker/.env \
  run --rm web \
  /opt/healthchecks/manage.py shell -c \
  "from django.contrib.auth.models import User; \
   email='admin@example.test'; password='adminpass123'; \
   u=User.objects.filter(email=email).first(); \
   u = u or User.objects.create_superuser('admin', email, password); \
   u.set_password(password); u.save(); print(email)"
```

## 2. Build the Provider

From this repository:

```bash
mkdir -p .dist
go build -o .dist/terraform-provider-healthchecks .
```

## 3. Use Terraform Dev Overrides

Create a local CLI config file:

```hcl
provider_installation {
  dev_overrides {
    "ehsanmsb/healthchecks" = "/ABSOLUTE/PATH/TO/terraform-provider-healthchecks/.dist"
  }
  direct {}
}
```

## 4. Run the Smoke Test

Use the example in [examples/local_smoke](/Users/snapp/Documents/Github/terraform-provider-healthchecks/examples/local_smoke/main.tf:1):

```bash
cd examples/local_smoke
TF_CLI_CONFIG_FILE=/path/to/dev.tfrc terraform init
TF_VAR_username=admin@example.test \
TF_VAR_password=adminpass123 \
TF_CLI_CONFIG_FILE=/path/to/dev.tfrc \
terraform apply
```

This exercises:

- `healthchecks_project`
- `healthchecks_check`
- `healthchecks_integration` with `webhook`
- `healthchecks_project_member`
