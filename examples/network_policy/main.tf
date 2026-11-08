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

# A reusable network policy, owned by the organisation. Ingress is restricted
# to the named peer projects, and egress to the listed CIDRs, which makes
# outbound traffic from the workload denied unless listed. The ports scope the
# rules to 443/TCP.
resource "bahriya_network_policy" "web_tier" {
  handle = "web-tier"
  name   = "Web tier ingress"

  ingress_peers = ["frontend", "gateway"]
  egress_cidrs  = ["10.0.0.0/8", "203.0.113.0/24"]

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

# Alternatively, scope it to a single container by referencing its handle:
#
# resource "bahriya_container" "api" {
#   # ... other fields ...
#   networking = {
#     external         = true
#     network_policies = [bahriya_network_policy.web_tier.handle]
#   }
# }

output "network_policy_id" {
  value = bahriya_network_policy.web_tier.id
}
