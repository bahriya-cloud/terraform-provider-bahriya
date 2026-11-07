resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1", "singapore-1"]
}

resource "bahriya_valkey" "catalog" {
  handle  = "catalog"
  name    = "Catalog"
  purpose = "cache"
  tier    = "sharded"
  memory  = 2048
  regions = ["falkenstein-1", "singapore-1"]
  project = bahriya_project.web.id

  topology = {
    size   = "hardened"
    shards = 3
  }

  # Free-form server settings. Values are strings.
  tuning = {
    maxmemory_policy = "allkeys-lfu"
    config = {
      "tcp-keepalive" = "300"
      "timeout"       = "0"
      "databases"     = "16"
      "loglevel"      = "notice"
    }
  }
}
