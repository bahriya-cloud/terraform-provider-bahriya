resource "bahriya_project" "shop" {
  handle  = "shop"
  name    = "Shop"
  regions = ["helsinki-1"]
}

resource "bahriya_mysql" "orders" {
  name      = "Orders DB"
  handle    = "orders-db"
  tier      = "single"
  cpu       = 500
  memory    = 1024
  project   = bahriya_project.shop.id
  regions   = ["helsinki-1"]
  databases = [{ name = "orders", username = "orders-app" }]

  persistence = {
    storage = 10
  }

  security = {
    tls = true
  }
}

data "bahriya_tls_bundle" "orders" {
  handle = bahriya_mysql.orders.security.tls_bundle
}

# The CA certificate clients use to verify the server, for example:
# terraform output -raw orders_ca > orders-db-ca.pem
output "orders_ca" {
  value = data.bahriya_tls_bundle.orders.ca
}
