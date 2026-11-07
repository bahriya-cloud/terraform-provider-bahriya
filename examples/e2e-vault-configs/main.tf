terraform {
  required_providers {
    bahriya = {
      source = "registry.terraform.io/bahriya-cloud/bahriya"
    }
  }
}

provider "bahriya" {}

# ===========================================================================
# Comprehensive end-to-end coverage.
#
# Exercises, in one apply/destroy cycle:
#   - data sources            (organisation, regions)
#   - project                 (multi-region)
#   - registry + secret       (+ their project attachments)
#   - all 5 vault item types  (+ project attachments + container mounts)
#   - all 4 config item types (+ project attachments + container mounts)
#   - network policy          (+ project attachment + container reference)
#   - memcached deployable
#   - HTTP container          (vault/config mounts, persistent volume,
#                              hostnames, env vars, secret env vars, netpol)
#   - path rules              (basic auth + rate limiting, on the HTTP container)
#   - worker container        (type = worker)
#   - cronjob container        (type = cronjob, schedule)
#
# Attribute names follow the provider v1 schema, which matches the Reis YAML
# shape: groups such as workload, scaling, networking, vault and configs, and
# underscored names such as mount_path.
#
# Run ./gen-fixtures.sh first: it writes the key material and the handle suffix
# into fixtures/, which this configuration reads.
# ===========================================================================

variable "fixtures_dir" {
  description = "Directory, relative to this configuration, holding the files written by gen-fixtures.sh."
  type        = string
  default     = "fixtures"
}

variable "registry_password" {
  type      = string
  sensitive = true
  default   = "e2e-dummy-registry-password"
}

variable "secret_value" {
  type      = string
  sensitive = true
  default   = "e2e-dummy-secret-value"
}

variable "admin_password" {
  type      = string
  sensitive = true
  default   = "e2e-dummy-admin-password"
}

locals {
  fixtures    = "${path.module}/${var.fixtures_dir}"
  suffix      = chomp(file("${local.fixtures}/suffix.txt"))
  region      = "helsinki-1"
  proj_handle = "tfe2e-${local.suffix}"
}

# ===========================================================================
# Data sources
# ===========================================================================

# NOTE: the bahriya_organisation data source is currently broken upstream — it
# calls GET /organisations/{id}, which the API does not expose (404); only the
# list endpoint exists. Omitted here until the provider data source is fixed.
data "bahriya_regions" "active" {
  status_filter = "active"
}

# ===========================================================================
# Project
# ===========================================================================

resource "bahriya_project" "e2e" {
  handle  = local.proj_handle
  name    = "TF E2E Comprehensive ${local.suffix}"
  regions = [local.region]
}

# A second project used as a real ingress peer for the network policy below.
# ingress_peers must reference project handles that exist in the organisation.
resource "bahriya_project" "peer" {
  handle  = "tfe2epeer-${local.suffix}"
  name    = "TF E2E Peer ${local.suffix}"
  regions = [local.region]
}

# ===========================================================================
# Registry + Secret (organisation-scoped) + project attachments
# ===========================================================================

resource "bahriya_registry" "main" {
  handle   = "reg-${local.suffix}"
  name     = "TF E2E Registry ${local.suffix}"
  server   = "ghcr.io"
  username = "e2e-user"
  password = var.registry_password
}

resource "bahriya_secret" "main" {
  handle = "sec-${local.suffix}"
  name   = "TF E2E Secret ${local.suffix}"
  value  = var.secret_value
}

resource "bahriya_project_registry_attachment" "main" {
  project_id = bahriya_project.e2e.id
  handle     = bahriya_registry.main.handle
}

resource "bahriya_project_secret_attachment" "main" {
  project_id = bahriya_project.e2e.id
  handle     = bahriya_secret.main.handle
}

# ===========================================================================
# Vault items (organisation-scoped)
# ===========================================================================

resource "bahriya_tls_bundle" "main" {
  handle = "tls-${local.suffix}"
  name   = "TF E2E TLS Bundle ${local.suffix}"
  ca     = file("${local.fixtures}/ca.pem")
  cert   = file("${local.fixtures}/server.pem")
  key    = file("${local.fixtures}/server.key")
}

resource "bahriya_x509_cert" "main" {
  handle = "x509-${local.suffix}"
  name   = "TF E2E X509 Cert ${local.suffix}"
  cert   = file("${local.fixtures}/x509.pem")
}

resource "bahriya_gpg_keypair" "main" {
  handle      = "gpg-${local.suffix}"
  name        = "TF E2E GPG Keypair ${local.suffix}"
  public_key  = file("${local.fixtures}/gpg.pub")
  private_key = file("${local.fixtures}/gpg.key")
}

resource "bahriya_ssh_keypair" "main" {
  handle      = "ssh-${local.suffix}"
  name        = "TF E2E SSH Keypair ${local.suffix}"
  public_key  = file("${local.fixtures}/ssh.pub")
  private_key = file("${local.fixtures}/ssh")
}

resource "bahriya_encryption_key" "main" {
  handle = "enc-${local.suffix}"
  name   = "TF E2E Encryption Key ${local.suffix}"
  format = "base64"
  key    = chomp(file("${local.fixtures}/encryption.key"))
}

# ===========================================================================
# Config items (organisation-scoped)
# ===========================================================================

resource "bahriya_env_file" "main" {
  handle  = "env-${local.suffix}"
  name    = "TF E2E Env File ${local.suffix}"
  content = file("${local.fixtures}/env-file.txt")
}

resource "bahriya_yaml_config" "main" {
  handle  = "yaml-${local.suffix}"
  name    = "TF E2E YAML Config ${local.suffix}"
  content = file("${local.fixtures}/config.yaml")
}

resource "bahriya_json_config" "main" {
  handle  = "json-${local.suffix}"
  name    = "TF E2E JSON Config ${local.suffix}"
  content = file("${local.fixtures}/config.json")
}

resource "bahriya_plain_config" "main" {
  handle  = "plain-${local.suffix}"
  name    = "TF E2E Plain Config ${local.suffix}"
  content = file("${local.fixtures}/config.plain")
}

# ===========================================================================
# Network policy (organisation-scoped) + project attachment
# ===========================================================================

resource "bahriya_network_policy" "main" {
  handle = "netpol-${local.suffix}"
  name   = "TF E2E Network Policy ${local.suffix}"

  ingress_peers = [bahriya_project.peer.handle]
  egress_cidrs  = ["10.0.0.0/8"]

  ports = [
    {
      port     = 443
      protocol = "TCP"
    },
  ]
}

resource "bahriya_project_network_policy_attachment" "main" {
  project_id = bahriya_project.e2e.id
  handle     = bahriya_network_policy.main.handle
}

# A SECOND policy attached at CONTAINER scope only (deliberately NOT project-
# attached). This is the #69 round-trip case: a container-scope
# `networking.network_policies` entry must apply and read back cleanly (no
# "element 0 has vanished"). Putting a project-attached policy on a container is
# refused with 409 (covered by the API integration tests), so the container
# references this container-scoped policy.
resource "bahriya_network_policy" "podscoped" {
  handle = "netpol-pod-${local.suffix}"
  name   = "TF E2E Pod-scoped Network Policy ${local.suffix}"

  egress_cidrs = ["10.1.0.0/16"]

  ports = [
    {
      port     = 8080
      protocol = "TCP"
    },
  ]
}

# ===========================================================================
# Project attachments for every vault + config kind
# ===========================================================================

resource "bahriya_project_tls_bundle_attachment" "main" {
  project_id = bahriya_project.e2e.id
  handle     = bahriya_tls_bundle.main.handle
}

resource "bahriya_project_x509_cert_attachment" "main" {
  project_id = bahriya_project.e2e.id
  handle     = bahriya_x509_cert.main.handle
}

resource "bahriya_project_gpg_keypair_attachment" "main" {
  project_id = bahriya_project.e2e.id
  handle     = bahriya_gpg_keypair.main.handle
}

resource "bahriya_project_ssh_keypair_attachment" "main" {
  project_id = bahriya_project.e2e.id
  handle     = bahriya_ssh_keypair.main.handle
}

resource "bahriya_project_encryption_key_attachment" "main" {
  project_id = bahriya_project.e2e.id
  handle     = bahriya_encryption_key.main.handle
}

resource "bahriya_project_env_file_attachment" "main" {
  project_id = bahriya_project.e2e.id
  handle     = bahriya_env_file.main.handle
}

resource "bahriya_project_yaml_config_attachment" "main" {
  project_id = bahriya_project.e2e.id
  handle     = bahriya_yaml_config.main.handle
}

resource "bahriya_project_json_config_attachment" "main" {
  project_id = bahriya_project.e2e.id
  handle     = bahriya_json_config.main.handle
}

resource "bahriya_project_plain_config_attachment" "main" {
  project_id = bahriya_project.e2e.id
  handle     = bahriya_plain_config.main.handle
}

# ===========================================================================
# Memcached deployable
# ===========================================================================

resource "bahriya_memcached" "cache" {
  handle  = "cache-${local.suffix}"
  name    = "TF E2E Cache ${local.suffix}"
  regions = [local.region]
  project = bahriya_project.e2e.id

  tuning = {
    memory = 512
  }
}

# ===========================================================================
# Valkey deployable: the single tier keeps the example inside a default project
# quota. security.auth is deliberately omitted: the platform default is true,
# so the instance must come up requiring its generated password.
# ===========================================================================

resource "bahriya_valkey" "kv" {
  handle  = "kv-${local.suffix}"
  name    = "TF E2E Valkey ${local.suffix}"
  tier    = "single"
  purpose = "cache"
  memory  = 256
  regions = [local.region]
  project = bahriya_project.e2e.id
}

# ===========================================================================
# HTTP container: exercises every container-side attachment, a persistent
# volume, env vars, a secret env var, a domain and a network policy.
# Persistent volumes rule out autoscaling, so scaling.replicas stays 1 with no
# maximum.
# ===========================================================================

resource "bahriya_container" "web" {
  handle  = "web-${local.suffix}"
  name    = "TF E2E Web ${local.suffix}"
  type    = "http"
  regions = [local.region]
  project = bahriya_project.e2e.id

  workload = {
    image       = "nginx:alpine"
    port        = "80"
    healthcheck = "/"
    cpu         = "100"
    memory      = "128"

    # Plain env vars
    env = {
      "APP_MODE" = "e2e"
    }

    # Secret-backed env var: env var name => secret handle
    secrets = {
      "APP_SECRET" = bahriya_secret.main.handle
    }
  }

  scaling = {
    replicas = "1"
  }

  networking = {
    # Custom domain
    domains = [
      {
        hostname    = "web-${local.suffix}.e2e.bahriya.test"
        wwwredirect = false
      },
    ]

    # Network policy at container scope: a policy that is not project-attached,
    # so it must round-trip (the #69 fix). The project-attached
    # bahriya_network_policy.main already applies to every container in the
    # project.
    network_policies = [bahriya_network_policy.podscoped.handle]
  }

  # Persistent storage
  storage = {
    persistent_volumes = [
      {
        handle     = "data"
        mount_path = "/var/lib/data"
        size_gb    = 1
      },
    ]
  }

  # Vault attachments
  vault = {
    tls_bundles = [
      { handle = bahriya_tls_bundle.main.handle, mount_path = "/etc/bahriya/tls" },
    ]
    x509_certs = [
      { handle = bahriya_x509_cert.main.handle, mount_path = "/etc/bahriya/x509" },
    ]
    gpg_keypairs = [
      { handle = bahriya_gpg_keypair.main.handle, mount_path = "/etc/bahriya/gpg" },
    ]
    ssh_keypairs = [
      { handle = bahriya_ssh_keypair.main.handle, mount_path = "/etc/bahriya/ssh" },
    ]
    encryption_keys = [
      { handle = bahriya_encryption_key.main.handle, mount_path = "/etc/bahriya/enc" },
    ]
  }

  # Config attachments
  configs = {
    env_files = [
      { handle = bahriya_env_file.main.handle, injection_method = "envFrom" },
    ]
    yaml_configs = [
      { handle = bahriya_yaml_config.main.handle, mount_path = "/etc/bahriya/cfg/yaml" },
    ]
    json_configs = [
      { handle = bahriya_json_config.main.handle, mount_path = "/etc/bahriya/cfg/json" },
    ]
    plain_configs = [
      { handle = bahriya_plain_config.main.handle, mount_path = "/etc/bahriya/cfg/plain" },
    ]
  }

  depends_on = [
    bahriya_project_network_policy_attachment.main,
    bahriya_project_secret_attachment.main,
    bahriya_project_tls_bundle_attachment.main,
    bahriya_project_x509_cert_attachment.main,
    bahriya_project_gpg_keypair_attachment.main,
    bahriya_project_ssh_keypair_attachment.main,
    bahriya_project_encryption_key_attachment.main,
    bahriya_project_env_file_attachment.main,
    bahriya_project_yaml_config_attachment.main,
    bahriya_project_json_config_attachment.main,
    bahriya_project_plain_config_attachment.main,
  ]
}

# ===========================================================================
# Path rules on the HTTP container
# ===========================================================================

resource "bahriya_path_rule" "admin" {
  container = bahriya_container.web.id
  handle    = "admin-${local.suffix}"
  path      = "/admin"
  priority  = 100

  security = {
    basic_auth_enabled = true
    basic_auth = [
      { username = "alice", password = var.admin_password },
    ]

    ip_allowlist = {
      enabled = true
      ips     = ["10.0.0.0/8"]
    }
  }
}

resource "bahriya_path_rule" "webhook" {
  container = bahriya_container.web.id
  handle    = "webhook-${local.suffix}"
  path      = "/webhook"

  security = {
    rate_limiting = {
      enabled    = true
      per_minute = 60
      per_hour   = 1000
    }
  }
}

# ===========================================================================
# Worker container (no ingress, no health check)
# ===========================================================================

resource "bahriya_container" "worker" {
  handle  = "worker-${local.suffix}"
  name    = "TF E2E Worker ${local.suffix}"
  type    = "worker"
  regions = [local.region]
  project = bahriya_project.e2e.id

  workload = {
    image  = "nginx:alpine"
    cpu    = "100"
    memory = "128"

    secrets = {
      "WORKER_SECRET" = bahriya_secret.main.handle
    }
  }

  scaling = {
    replicas = "1"
  }

  depends_on = [bahriya_project_secret_attachment.main]
}

# ===========================================================================
# Cronjob container (runs on a schedule)
# ===========================================================================

resource "bahriya_container" "cron" {
  handle  = "cron-${local.suffix}"
  name    = "TF E2E Cron ${local.suffix}"
  type    = "cronjob"
  regions = [local.region]
  project = bahriya_project.e2e.id

  workload = {
    image   = "busybox:latest"
    command = ["sh", "-c", "echo e2e-tick"]
    cpu     = "100"
    memory  = "128"
  }

  schedule = {
    schedule = "*/5 * * * *"
  }
}

# ===========================================================================
# Outputs
# ===========================================================================

output "active_regions" {
  value = [for r in data.bahriya_regions.active.regions : r.id]
}

output "project_id" {
  value = bahriya_project.e2e.id
}

output "registry_id" {
  value = bahriya_registry.main.id
}

output "secret_id" {
  value = bahriya_secret.main.id
}

output "network_policy_id" {
  value = bahriya_network_policy.main.id
}

output "memcached_id" {
  value = bahriya_memcached.cache.id
}

output "memcached_status" {
  value = bahriya_memcached.cache.status
}

output "web_container_id" {
  value = bahriya_container.web.id
}

output "web_container_status" {
  value = bahriya_container.web.status
}

output "worker_container_status" {
  value = bahriya_container.worker.status
}

output "cron_container_status" {
  value = bahriya_container.cron.status
}

output "suffix" {
  value = local.suffix
}
