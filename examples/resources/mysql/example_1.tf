resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_mysql" "orders" {
  handle  = "orders-db"
  name    = "Orders DB"
  tier    = "single"
  cpu     = 1000
  memory  = 2048
  regions = ["falkenstein-1"]
  project = bahriya_project.web.id

  persistence = {
    storage = 20
  }

  databases = [
    { name = "orders", username = "orders-app" },
  ]
}
