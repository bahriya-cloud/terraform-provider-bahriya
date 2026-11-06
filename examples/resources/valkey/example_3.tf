resource "bahriya_valkey" "shared" {
  handle        = "shared-cache"
  name          = "Shared Cache"
  purpose       = "cache"
  tier          = "single"
  memorymb      = 512
  activeregions = ["falkenstein-1"]
  project       = bahriya_project.web.id

  externalenabled = true
  tlsenabled      = true
  authenabled     = true
  allowedips      = ["198.51.100.0/24"]

  hostnames = [
    { hostname = "cache.example.com" },
  ]
}
