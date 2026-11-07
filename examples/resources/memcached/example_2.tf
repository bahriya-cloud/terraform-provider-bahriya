resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_memcached" "api_cache" {
  handle  = "api-cache"
  name    = "API Response Cache"
  regions = ["falkenstein-1"]
  project = bahriya_project.web.id

  topology = {
    nodes = 3
  }

  tuning = {
    memory           = 1024
    max_connections  = 2048
    threads          = 8
    max_item_size_mb = 5
  }
}
