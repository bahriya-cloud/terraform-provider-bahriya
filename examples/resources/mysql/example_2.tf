resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_mysql" "shop" {
  handle  = "shop-db"
  name    = "Shop DB"
  tier    = "replicated"
  cpu     = 2000
  memory  = 8192
  regions = ["falkenstein-1"]
  project = bahriya_project.web.id

  topology = {
    replicas = 2
  }

  persistence = {
    storage = 100
  }

  databases = [
    { name = "catalog", username = "catalog-app" },
    { name = "reporting", username = "reporting-ro" },
  ]

  # Free-form server settings. Values are strings.
  tuning = {
    config = {
      "max_connections" = "500"
      "slow_query_log"  = "true"
      "long_query_time" = "2"
    }
  }

  backup = {
    backup_schedule = "0 3 * * *"
  }
}
