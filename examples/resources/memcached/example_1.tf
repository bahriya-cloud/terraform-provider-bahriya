resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_memcached" "session" {
  handle  = "session-cache"
  name    = "Session Cache"
  regions = ["falkenstein-1"]
  project = bahriya_project.web.id

  tuning = {
    memory = 512
  }
}
