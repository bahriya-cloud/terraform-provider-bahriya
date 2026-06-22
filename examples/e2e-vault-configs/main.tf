terraform {
  required_providers {
    bahriya = {
      source = "registry.terraform.io/bahriya-cloud/bahriya"
    }
  }
}

provider "bahriya" {}

locals {
  suffix    = chomp(file("${path.module}/fixtures/suffix.txt"))
  region    = "helsinki-1"
  proj_handle = "tfe2e-${local.suffix}"
}

# ---------- Project ----------

resource "bahriya_project" "e2e" {
  handle  = local.proj_handle
  name    = "TF E2E Vault Configs ${local.suffix}"
  regions = [local.region]
}

# ===========================================================================
# Vault items (organisation-scoped)
# ===========================================================================

resource "bahriya_tls_bundle" "main" {
  handle = "tls-${local.suffix}"
  name   = "TF E2E TLS Bundle Renamed ${local.suffix}"
  ca     = file("${path.module}/fixtures/ca.pem")
  cert   = file("${path.module}/fixtures/server.pem")
  key    = file("${path.module}/fixtures/server.key")
}

resource "bahriya_x509_cert" "main" {
  handle = "x509-${local.suffix}"
  name   = "TF E2E X509 Cert ${local.suffix}"
  cert   = file("${path.module}/fixtures/x509.pem")
}

resource "bahriya_gpg_keypair" "main" {
  handle      = "gpg-${local.suffix}"
  name        = "TF E2E GPG Keypair ${local.suffix}"
  public_key  = file("${path.module}/fixtures/gpg.pub")
  private_key = file("${path.module}/fixtures/gpg.key")
}

resource "bahriya_ssh_keypair" "main" {
  handle      = "ssh-${local.suffix}"
  name        = "TF E2E SSH Keypair ${local.suffix}"
  public_key  = file("${path.module}/fixtures/ssh.pub")
  private_key = file("${path.module}/fixtures/ssh")
}

resource "bahriya_encryption_key" "main" {
  handle    = "enc-${local.suffix}"
  name      = "TF E2E Encryption Key ${local.suffix}"
  algorithm = "AES-256"
  format    = "base64"
  key       = chomp(file("${path.module}/fixtures/encryption.key"))
}

# ===========================================================================
# Config items (organisation-scoped)
# ===========================================================================

resource "bahriya_env_file" "main" {
  handle  = "env-${local.suffix}"
  name    = "TF E2E Env File ${local.suffix}"
  content = file("${path.module}/fixtures/env-file.txt")
}

resource "bahriya_yaml_config" "main" {
  handle  = "yaml-${local.suffix}"
  name    = "TF E2E YAML Config ${local.suffix}"
  content = file("${path.module}/fixtures/config.yaml")
}

resource "bahriya_json_config" "main" {
  handle  = "json-${local.suffix}"
  name    = "TF E2E JSON Config ${local.suffix}"
  content = file("${path.module}/fixtures/config.json")
}

resource "bahriya_plain_config" "main" {
  handle  = "plain-${local.suffix}"
  name    = "TF E2E Plain Config ${local.suffix}"
  content = file("${path.module}/fixtures/config.plain")
}

# ===========================================================================
# Project attachments — one of each kind
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

# json_config attachment removed for Round D - should succeed since container no longer uses json

resource "bahriya_project_plain_config_attachment" "main" {
  project_id = bahriya_project.e2e.id
  handle     = bahriya_plain_config.main.handle
}

# ===========================================================================
# Container — exercises every container-side attachment + persistent storage.
# Storage forbids autoscaling per the schema; keep autoscalingminreplicas=1
# and do not set max.
# ===========================================================================

resource "bahriya_container" "e2e" {
  handle    = "ctr4-${local.suffix}"
  name      = "TF E2E Container ${local.suffix}"
  image     = "nginx:alpine"
  mincpu    = "100"
  minmemory = "128"

  containerport   = "80"
  healthcheckpath = "/"

  autoscalingminreplicas = "1"

  activeregions = [local.region]
  project       = bahriya_project.e2e.id

  # Persistent storage
  persistentvolumes = [
    {
      handle    = "data"
      mountpath = "/var/lib/data"
      sizegb    = 1
    },
  ]

  # Vault attachments to container
  tls_bundles = [
    {
      handle    = bahriya_tls_bundle.main.handle
      mountpath = "/etc/bahriya/tls"
    },
  ]

  x509_certs = [
    {
      handle    = bahriya_x509_cert.main.handle
      mountpath = "/etc/bahriya/x509"
    },
  ]

  gpg_keypairs = [
    {
      handle    = bahriya_gpg_keypair.main.handle
      mountpath = "/etc/bahriya/gpg"
    },
  ]

  ssh_keypairs = [
    {
      handle    = bahriya_ssh_keypair.main.handle
      mountpath = "/etc/bahriya/ssh"
    },
  ]

  encryption_keys = [
    {
      handle    = bahriya_encryption_key.main.handle
      mountpath = "/etc/bahriya/enc"
    },
  ]

  # Config attachments to container
  env_files = [
    {
      handle          = bahriya_env_file.main.handle
      injectionmethod = "envFrom"
    },
  ]

  yaml_configs = [
    {
      handle    = bahriya_yaml_config.main.handle
      mountpath = "/etc/bahriya/cfg/yaml"
    },
  ]

  # json_configs explicitly emptied for detach-test (Round B)
  json_configs = []

  plain_configs = [
    {
      handle    = bahriya_plain_config.main.handle
      mountpath = "/etc/bahriya/cfg/plain"
    },
  ]

  # Project attachments must exist before the container references them.
  depends_on = [
    bahriya_project_tls_bundle_attachment.main,
    bahriya_project_x509_cert_attachment.main,
    bahriya_project_gpg_keypair_attachment.main,
    bahriya_project_ssh_keypair_attachment.main,
    bahriya_project_encryption_key_attachment.main,
    bahriya_project_env_file_attachment.main,
    bahriya_project_yaml_config_attachment.main,
    # bahriya_project_json_config_attachment.main, # removed for Round D

    bahriya_project_plain_config_attachment.main,
  ]
}

# ---------- Outputs ----------

output "project_id" {
  value = bahriya_project.e2e.id
}

output "container_id" {
  value = bahriya_container.e2e.id
}

output "container_status" {
  value = bahriya_container.e2e.status
}

output "suffix" {
  value = local.suffix
}
