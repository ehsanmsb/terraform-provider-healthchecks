# Roadmap

This document collects practical next steps for the Healthchecks Terraform provider. Each item here is a good candidate to split into a standalone GitHub issue.

## High Priority

- Add direct API key authentication to the provider
  This reduces reliance on session login and makes automation friendlier for existing Healthchecks projects.

- Add data sources
  Good first targets are project, check, and integration lookup data sources.

- Expand integration support beyond `webhook` and `email`
  Good next candidates are integrations with straightforward project-scoped form flows such as `ntfy`, `gotify`, `matrix`, `shell`, and `webhook`-adjacent types.

- Improve acceptance coverage
  Add focused acceptance tests for project key toggles, integration CRUD, check channel attachment, and import behavior.

## Medium Priority

- Add richer import support
  Imported resources should preserve stable state more cleanly without unnecessary key regeneration or channel reordering churn.

- Add additional project resources
  Potential targets include project-scoped read-only views and any other project-management workflows that map cleanly to current upstream views.

- Improve integration modularity
  Split integration implementations by type to make future support additions easier to review and test.

- Add retry and recovery improvements around web-form flows
  Especially around redirects, key regeneration, and transient local/self-hosted failures.

## Nice To Have

- Add examples for common self-hosted deployment patterns
  Include examples for direct API key auth once supported and for multi-project usage.

- Improve docs generation and release docs flow
  Keep `tfplugindocs` output and release documentation tightly aligned with CI.

- Add more observability around provider behavior
  Better debug logging around authenticated web flows would make troubleshooting easier without leaking secrets.
