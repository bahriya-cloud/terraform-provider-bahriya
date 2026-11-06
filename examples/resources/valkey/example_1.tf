resource "bahriya_valkey" "sessions" {
  handle        = "session-cache"
  name          = "Session Cache"
  purpose       = "cache"
  tier          = "single"
  memorymb      = 256
  activeregions = ["falkenstein-1"]
  project       = bahriya_project.web.id
}
