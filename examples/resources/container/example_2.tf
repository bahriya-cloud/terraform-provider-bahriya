resource "bahriya_project" "web" {
  name    = "Web"
  handle  = "web"
  regions = ["falkenstein-1"]
}

resource "bahriya_container" "queue" {
  name    = "Queue Worker"
  handle  = "queue-worker"
  type    = "worker"
  project = bahriya_project.web.id
  regions = ["falkenstein-1"]

  workload = {
    image   = "ghcr.io/myorg/app:1.0.0"
    cpu     = "200"
    memory  = "256"
    command = ["/usr/bin/php"]
    args    = ["artisan", "queue:work", "--tries=3"]
  }

  scaling = {
    replicas = "1"
  }
}
