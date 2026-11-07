resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_valkey" "shared" {
  handle  = "shared-cache"
  name    = "Shared Cache"
  purpose = "cache"
  tier    = "single"
  memory  = 512
  regions = ["falkenstein-1"]
  project = bahriya_project.web.id

  security = {
    tls  = true
    auth = true
  }

  exposure = {
    external    = true
    allowed_ips = ["198.51.100.0/24"]
    hostnames = [
      { hostname = "cache.example.com" },
    ]
  }
}
