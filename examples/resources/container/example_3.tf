resource "bahriya_project" "web" {
  name    = "Web"
  handle  = "web"
  regions = ["falkenstein-1"]
}

resource "bahriya_container" "nightly" {
  name    = "Nightly Reports"
  handle  = "nightly-reports"
  type    = "cronjob"
  project = bahriya_project.web.id
  regions = ["falkenstein-1"]

  workload = {
    image   = "ghcr.io/myorg/reports:1.0.0"
    cpu     = 100
    memory  = 256
    command = ["/usr/bin/php"]
    args    = ["artisan", "reports:nightly"]
  }

  scaling = {
    replicas = 1
  }

  schedule = {
    schedule    = "0 2 * * *"
    tz          = "Europe/London"
    concurrency = "Forbid"
  }
}
