terraform {
  required_providers {
    bahriya = {
      source = "registry.terraform.io/bahriya-cloud/bahriya"
    }
  }
}

provider "bahriya" {}

resource "bahriya_registry" "test" {
  handle   = "tf-test-reg"
  name     = "TF Test Registry"
  server   = "ghcr.io"
  username = "testuser"
  password = var.registry_password
}

resource "bahriya_secret" "test" {
  handle = "tf-test-secret"
  name   = "TF Test Secret"
  value  = var.secret_value
}

variable "registry_password" {
  type      = string
  sensitive = true
}

variable "secret_value" {
  type      = string
  sensitive = true
}

output "registry_id" {
  value = bahriya_registry.test.id
}

output "secret_id" {
  value = bahriya_secret.test.id
}
