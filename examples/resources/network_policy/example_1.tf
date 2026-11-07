resource "bahriya_network_policy" "web_tier" {
  name   = "Web tier"
  handle = "web-tier"

  # Ingress: only these projects of the same organisation may reach the targets.
  ingress_peers = ["frontend", "gateway"]

  # Egress (L3/L4): outbound traffic is restricted to these CIDRs on the listed ports.
  egress_cidrs = ["10.0.0.0/8", "203.0.113.0/24"]

  ports = [
    { port = 443, protocol = "TCP" },
    { port = 53, protocol = "UDP" },
  ]
}

# L7 egress: allow specific domains. Setting egress_fqdns enables enable_l7,
# which adds a per-replica charge for every container the policy is applied to.
resource "bahriya_network_policy" "notifications" {
  name         = "Notifications to partner"
  handle       = "notifications-to-partner"
  egress_fqdns = ["api.partner.example.com"]
}
