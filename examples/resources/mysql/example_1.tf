resource "bahriya_mysql" "orders" {
  handle        = "orders-db"
  name          = "Orders DB"
  tier          = "single"
  cpumillicores = 1000
  memorymb      = 2048
  storagegb     = 20
  activeregions = ["falkenstein-1"]
  project       = bahriya_project.web.id

  databases = [
    { name = "orders", username = "orders-app" },
  ]
}
