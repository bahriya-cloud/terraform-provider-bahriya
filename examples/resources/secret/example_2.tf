variable "db_password" {
  type      = string
  sensitive = true
}

resource "bahriya_project" "shop" {
  name    = "Shop"
  handle  = "shop"
  regions = ["helsinki-1"]
}

resource "bahriya_secret" "db_password" {
  name   = "Database password"
  handle = "db-password"
  value  = var.db_password
}

resource "bahriya_container" "api" {
  name    = "API"
  handle  = "api"
  project = bahriya_project.shop.id
  regions = ["helsinki-1"]

  workload = {
    image  = "registry.example.com/shop/api:1.0.0"
    cpu    = 250
    memory = 256
    secrets = {
      DATABASE_PASSWORD = bahriya_secret.db_password.handle
    }
  }
}
