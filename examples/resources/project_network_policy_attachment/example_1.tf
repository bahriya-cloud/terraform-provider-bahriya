resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_network_policy" "web_tier" {
  handle        = "web-tier"
  name          = "Web tier"
  ingress_peers = ["api"]
}

resource "bahriya_project_network_policy_attachment" "web_web_tier" {
  project_id = bahriya_project.web.id
  handle     = bahriya_network_policy.web_tier.handle
}
