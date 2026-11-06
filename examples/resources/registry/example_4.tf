resource "bahriya_container" "api" {
  handle   = "api"
  name     = "API Server"
  image    = "ghcr.io/myorg/api:1.0.0"
  registry = bahriya_registry.ghcr.handle
  # ...
}
