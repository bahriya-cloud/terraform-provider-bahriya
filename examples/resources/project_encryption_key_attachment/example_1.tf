variable "data_key" {
  type      = string
  sensitive = true
}

resource "bahriya_project" "web" {
  handle  = "web"
  name    = "Web"
  regions = ["falkenstein-1"]
}

resource "bahriya_encryption_key" "data_key" {
  handle = "data-key"
  name   = "Data encryption key"
  format = "base64"
  key    = var.data_key
}

resource "bahriya_project_encryption_key_attachment" "web_encryption_key" {
  project_id = bahriya_project.web.id
  handle     = bahriya_encryption_key.data_key.handle
}
