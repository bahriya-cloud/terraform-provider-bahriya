resource "bahriya_container" "api" {
  handle          = "rate-limited-api"
  name            = "Rate Limited API"
  image           = "nginx:alpine"
  containerport   = "80"
  healthcheckpath = "/"
  mincpu          = "100"
  minmemory       = "128"

  autoscalingminreplicas = "1"
  activeregions          = ["falkenstein-1"]
  project                = bahriya_project.web.id

  ratelimitingenabled          = true
  ratelimitingrequestspersecond = 10
  ratelimitingrequestsperminute = 300

  ipwhitelistenabled = true
  ipwhitelist        = ["203.0.113.0/24", "198.51.100.0/24"]
}
