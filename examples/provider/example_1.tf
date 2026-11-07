variable "bahriya_token" {
  type      = string
  sensitive = true
}

variable "bahriya_org_id" {
  type = string
}

provider "bahriya" {
  token           = var.bahriya_token
  organisation_id = var.bahriya_org_id
}
