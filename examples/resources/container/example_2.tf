resource "bahriya_container" "queue" {
  handle = "queue-worker"
  name   = "Queue Worker"
  image  = "ghcr.io/myorg/app:1.0.0"
  type   = "worker"

  mincpu    = "200"
  minmemory = "256"

  autoscalingminreplicas = "1"
  activeregions          = ["falkenstein-1"]
  project                = bahriya_project.web.id

  command = ["/usr/bin/php"]
  args    = ["artisan", "queue:work", "--tries=3"]
}
