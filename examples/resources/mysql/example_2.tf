resource "bahriya_mysql" "shop" {
  handle        = "shop-db"
  name          = "Shop DB"
  tier          = "replicated"
  replicas      = 2
  cpumillicores = 2000
  memorymb      = 8192
  storagegb     = 100
  activeregions = ["falkenstein-1"]
  project       = bahriya_project.web.id

  databases = [
    { name = "catalog", username = "catalog-app" },
    { name = "reporting", username = "reporting-ro" },
  ]

  config = {
    "max_connections" = "500"
    "slow_query_log"  = "true"
    "long_query_time" = "2"
  }

  backupschedule = "0 3 * * *"
}
