variable "prod_org_id" {
  type = string
}

variable "staging_org_id" {
  type = string
}

# One provider configuration per organisation. Resources choose one with
# `provider = bahriya.prod`.
provider "bahriya" {
  alias           = "prod"
  organisation_id = var.prod_org_id
}

provider "bahriya" {
  alias           = "staging"
  organisation_id = var.staging_org_id
}
