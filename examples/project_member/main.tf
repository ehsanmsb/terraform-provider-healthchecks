resource "healthchecks_project" "example" {
  name = "Team Project"
}

resource "healthchecks_project_member" "member" {
  project_id = healthchecks_project.example.id
  email      = "teammate@example.com"
  role       = "m"
}
