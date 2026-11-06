variable "db_password" {
  type      = string
  sensitive = true
}

resource "bahriya_secret" "db_password" {
  name   = "Database password"
  handle = "db-password"
  value  = var.db_password
}
