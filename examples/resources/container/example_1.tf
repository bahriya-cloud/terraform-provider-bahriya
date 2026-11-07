variable "ghcr_username" {
  type = string
}

variable "ghcr_token" {
  type      = string
  sensitive = true
}

variable "db_password" {
  type      = string
  sensitive = true
}

variable "tls_ca" {
  type = string
}

variable "tls_cert" {
  type = string
}

variable "tls_key" {
  type      = string
  sensitive = true
}

resource "bahriya_project" "web" {
  name    = "Web"
  handle  = "web"
  regions = ["falkenstein-1", "virginia-1"]
}

resource "bahriya_registry" "ghcr" {
  name     = "GitHub Container Registry"
  handle   = "ghcr"
  server   = "ghcr.io"
  username = var.ghcr_username
  password = var.ghcr_token
}

resource "bahriya_secret" "db_password" {
  name   = "Database password"
  handle = "db-password"
  value  = var.db_password
}

resource "bahriya_tls_bundle" "edge" {
  name   = "Edge certificate"
  handle = "edge"
  ca     = var.tls_ca
  cert   = var.tls_cert
  key    = var.tls_key
}

resource "bahriya_network_policy" "web_tier" {
  name   = "Web tier"
  handle = "web-tier"
}

resource "bahriya_container" "api" {
  name    = "API Server"
  handle  = "api"
  project = bahriya_project.web.id
  regions = ["falkenstein-1", "virginia-1"]

  workload = {
    image       = "ghcr.io/myorg/api:1.2.3"
    registry    = bahriya_registry.ghcr.id
    port        = "3000"
    healthcheck = "/healthz"
    cpu         = "250"
    memory      = "256"

    env = {
      "NODE_ENV" = "production"
    }

    # Environment variable name = secret handle.
    secrets = {
      "DATABASE_PASSWORD" = bahriya_secret.db_password.handle
    }
  }

  scaling = {
    replicas     = "2"
    max_replicas = "5"
  }

  networking = {
    domains = [
      { hostname = "api.example.com" },
    ]

    # A network policy listed here applies to this container only.
    network_policies = [bahriya_network_policy.web_tier.handle]
  }

  # Vault and config items attached to the project are mounted as files.
  vault = {
    tls_bundles = [
      { handle = bahriya_tls_bundle.edge.handle, mount_path = "/etc/tls" },
    ]
  }
}
