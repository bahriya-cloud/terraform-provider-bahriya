resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_valkey" "sessions" {
  handle  = "session-cache"
  name    = "Session Cache"
  purpose = "cache"
  tier    = "single"
  memory  = 256
  regions = ["falkenstein-1"]
  project = bahriya_project.web.id
}
