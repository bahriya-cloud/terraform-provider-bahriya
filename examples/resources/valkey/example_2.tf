resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_valkey" "queue" {
  handle  = "job-queue"
  name    = "Job Queue"
  purpose = "store"
  tier    = "ha"
  memory  = 1024
  regions = ["falkenstein-1"]
  project = bahriya_project.web.id

  topology = {
    size = "standard"
  }

  persistence = {
    storage = 10
  }

  backup = {
    backup          = true
    backup_schedule = "0 3 * * *"
  }
}
