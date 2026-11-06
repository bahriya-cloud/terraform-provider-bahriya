resource "bahriya_registry" "ghcr" {
  # ... bahriya_registry configuration ...
}

resource "bahriya_project_registry_attachment" "web_registry" {
  project_id = bahriya_project.web.id
  handle     = bahriya_registry.ghcr.handle
}
