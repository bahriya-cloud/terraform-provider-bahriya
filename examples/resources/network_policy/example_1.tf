resource "bahriya_network_policy" "web_tier" {
  handle = "web-tier"
  name   = "Web tier"

  # Ingress: only these same-org projects may reach the targets.
  ingresspeers = ["frontend", "gateway"]

  # Egress (L3/L4): outbound restricted to these CIDRs on the listed ports.
  egresscidrs = ["10.0.0.0/8", "203.0.113.0/24"]

  ports = [
    { port = 443, protocol = "TCP" },
    { port = 53, protocol = "UDP" },
  ]
}

# L7 egress: allow specific domains. Setting egressfqdns auto-enables l7enabled,
# which adds a per-replica charge for every container the policy is applied to.
resource "bahriya_network_policy" "notifications" {
  handle       = "notifications-to-partner"
  name         = "Notifications to partner"
  egressfqdns  = ["api.partner.example.com"]
}
