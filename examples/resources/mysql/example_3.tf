resource "bahriya_mysql" "ledger" {
  handle            = "ledger-db"
  name              = "Ledger DB"
  tier              = "ha"
  replicas          = 3
  cpumillicores     = 4000
  memorymb          = 16384
  storagegb         = 250
  encryptionenabled = true
  activeregions     = ["falkenstein-1"]
  project           = bahriya_project.web.id

  databases = [
    { name = "ledger", username = "ledger-app" },
  ]

  tlsenabled       = true
  tlsvalidityyears = 1
  externalenabled  = true
  allowedips       = ["198.51.100.0/24"]

  # Temporary browser access, switched off automatically after 8 hours.
  clientenabled  = true
  clientttlhours = 8

  hostnames = [
    { hostname = "db.example.com" },
  ]
}

output "ledger_port" {
  value = bahriya_mysql.ledger.externalport
}
