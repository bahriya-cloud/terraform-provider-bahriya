resource "bahriya_container" "nightly" {
  handle   = "nightly-reports"
  name     = "Nightly Reports"
  image    = "ghcr.io/myorg/reports:1.0.0"
  type     = "cronjob"
  schedule = "0 2 * * *"
  timezone = "Europe/London"

  concurrencypolicy = "Forbid"

  mincpu    = "100"
  minmemory = "256"

  autoscalingminreplicas = "1"
  activeregions          = ["falkenstein-1"]
  project                = bahriya_project.web.id

  command = ["/usr/bin/php"]
  args    = ["artisan", "reports:nightly"]
}
