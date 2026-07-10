terraform {
  required_providers {
    bahriya = {
      source = "registry.terraform.io/bahriya-cloud/bahriya"
    }
  }
}

provider "bahriya" {}

data "bahriya_regions" "active" {
  status_filter = "active"
}

resource "bahriya_project" "production" {
  handle  = "netpol-demo"
  name    = "Network Policy Demo"
  regions = [data.bahriya_regions.active.regions[0].id]
}

# A reusable, org-scoped network policy. Ingress is restricted to the named
# peer projects; egress is restricted to the listed CIDRs (which flips the
# workload to deny-by-default outbound). Ports scope the rules to 443/TCP.
resource "bahriya_network_policy" "web_tier" {
  handle = "web-tier"
  name   = "Web tier ingress"

  ingresspeers = ["frontend", "gateway"]
  egresscidrs  = ["10.0.0.0/8", "203.0.113.0/24"]

  ports = [
    {
      port     = 443
      protocol = "TCP"
    },
  ]
}

# Apply the policy to every workload in the project.
resource "bahriya_project_network_policy_attachment" "web_tier" {
  project_id = bahriya_project.production.id
  handle     = bahriya_network_policy.web_tier.handle
}

# Alternatively, scope it to a single container by referencing the handle:
#
# resource "bahriya_container" "api" {
#   # ... other fields ...
#   networkpolicies = [bahriya_network_policy.web_tier.handle]
# }

output "network_policy_id" {
  value = bahriya_network_policy.web_tier.id
}
