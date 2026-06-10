terraform {
  required_providers {
    bahriya = {
      source = "registry.terraform.io/bahriya-cloud/bahriya"
    }
  }
}

provider "bahriya" {}

resource "bahriya_project" "test" {
  handle  = "tf-project-example"
  name    = "TF Project Example"
  regions = ["falkenstein-1"]
}

output "project_id" {
  value = bahriya_project.test.id
}

output "project_handle" {
  value = bahriya_project.test.handle
}
