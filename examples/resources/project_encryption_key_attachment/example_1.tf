resource "bahriya_encryption_key" "data_key" {
  # ... bahriya_encryption_key configuration ...
}

resource "bahriya_project_encryption_key_attachment" "web_encryption_key" {
  project_id = bahriya_project.web.id
  handle     = bahriya_encryption_key.data_key.handle
}
