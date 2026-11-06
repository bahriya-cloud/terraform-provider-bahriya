resource "bahriya_memcached" "api_cache" {
  handle         = "api-cache"
  name           = "API Response Cache"
  memorymb       = 1024
  nodes          = 3
  maxconnections = 2048
  threads        = 8
  maxitemsizemb  = 5
  activeregions  = ["falkenstein-1"]
  project        = bahriya_project.web.id
}
