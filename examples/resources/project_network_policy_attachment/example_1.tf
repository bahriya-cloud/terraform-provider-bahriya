resource "bahriya_network_policy" "web_tier" {
  # ... bahriya_network_policy configuration ...
}

resource "bahriya_project_network_policy_attachment" "web_web_tier" {
  project_id = bahriya_project.web.id
  handle     = bahriya_network_policy.web_tier.handle
}
