resource "healthchecks_project" "example" {
  name = "Checks Project"
}

resource "healthchecks_check" "job" {
  project_id = healthchecks_project.example.id
  name       = "nightly-job"
  slug       = "nightly-job"
  timeout    = 3600
  grace      = 300
  tags       = ["batch", "nightly"]
}
