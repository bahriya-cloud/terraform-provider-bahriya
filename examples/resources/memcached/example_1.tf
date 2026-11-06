resource "bahriya_memcached" "session" {
  handle        = "session-cache"
  name          = "Session Cache"
  memorymb      = 512
  activeregions = ["falkenstein-1"]
  project       = bahriya_project.web.id
}
