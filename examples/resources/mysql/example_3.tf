resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_mysql" "ledger" {
  handle  = "ledger-db"
  name    = "Ledger DB"
  tier    = "ha"
  cpu     = 4000
  memory  = 16384
  regions = ["falkenstein-1"]
  project = bahriya_project.web.id

  topology = {
    replicas = 3
  }

  persistence = {
    storage = 250
  }

  databases = [
    { name = "ledger", username = "ledger-app" },
  ]

  security = {
    encryption         = true
    tls                = true
    tls_validity_years = 1
  }

  exposure = {
    external    = true
    allowed_ips = ["198.51.100.0/24"]
    hostnames = [
      { hostname = "db.example.com" },
    ]
  }

  # Temporary browser access, switched off automatically after 8 hours.
  client = {
    phpmyadmin     = true
    phpmyadmin_ttl = 8
  }
}

output "ledger_port" {
  value = bahriya_mysql.ledger.exposure.external_port
}
