# Healthchecks Endpoint Notes

Inspected upstream repository: `healthchecks/healthchecks` commit `a5708269858d6fcd7b9934161aaceba13250d176`.

Relevant routes and behaviors:

- Login form: `POST /accounts/login/` with `action=login`, `email`, `password`, CSRF token.
- Project create: `POST /projects/add/` with `name`.
- Project settings/read/update/team/API key page: `GET|POST /projects/<uuid:code>/settings/`.
- Project delete: `POST /projects/<uuid:code>/remove/`.
- RW API key create/revoke via project settings form:
  - `create_key=api_key`
  - `revoke_key=api_key`
- Team members managed on the same project settings form:
  - invite: `invite_team_member=1`, `email`, `role`
  - remove: `remove_team_member=1`, `email`
- Check Management API v3:
  - list/create: `/api/v3/checks/`
  - read/update/delete: `/api/v3/checks/<uuid:code>`
  - channels list: `/api/v3/channels/`
  - auth header: `X-Api-Key`
- Webhook integration create/edit:
  - create: `GET|POST /projects/<uuid:code>/add_webhook/`
  - edit: `GET|POST /integrations/<uuid:code>/edit/`
  - remove: `POST /integrations/<uuid:code>/remove/`

Implementation note:

- Checks use the public Management API v3.
- Projects, integrations, and project members use authenticated web forms because upstream does not currently expose equivalent public project-management endpoints.
- The provider may generate a new project RW API key when it cannot recover a plaintext key from state or the current response body. This is necessary because Healthchecks stores hashed API keys server-side.
