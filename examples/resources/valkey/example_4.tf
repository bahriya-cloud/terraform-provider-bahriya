resource "bahriya_valkey" "catalog" {
  handle        = "catalog"
  name          = "Catalog"
  purpose       = "cache"
  tier          = "sharded"
  size          = "hardened"
  shards        = 3
  memorymb      = 2048
  activeregions = ["falkenstein-1", "singapore-1"]
  project       = bahriya_project.web.id

  maxmemorypolicy = "allkeys-lfu"

  config = {
    "tcp-keepalive" = "300"
    "timeout"       = "0"
    "databases"     = "16"
    "loglevel"      = "notice"
  }
}
