resource "bahriya_registry" "docker_hub" {
  handle   = "docker-hub"
  name     = "Docker Hub"
  server   = "registry-1.docker.io"
  username = var.docker_username
  password = var.docker_password
}
