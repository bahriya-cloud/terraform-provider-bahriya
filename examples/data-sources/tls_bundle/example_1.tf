resource "bahriya_mysql" "orders" {
  name          = "Orders DB"
  handle        = "orders-db"
  tier          = "single"
  cpumillicores = 500
  memorymb      = 1024
  storagegb     = 10
  project       = bahriya_project.shop.id
  activeregions = ["helsinki-1"]
  databases     = [{ name = "orders", username = "orders-app" }]
  tlsenabled    = true
}

data "bahriya_tls_bundle" "orders" {
  handle = bahriya_mysql.orders.tlsbundle
}

# The CA certificate clients use to verify the server, for example:
# terraform output -raw orders_ca > orders-db-ca.pem
output "orders_ca" {
  value = data.bahriya_tls_bundle.orders.ca
}
