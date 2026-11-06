resource "bahriya_container" "api" {
  handle          = "api"
  name            = "API Server"
  image           = "ghcr.io/myorg/api:1.2.3"
  containerport   = "3000"
  healthcheckpath = "/healthz"
  mincpu          = "250"
  minmemory       = "256"

  autoscalingminreplicas = "2"
  autoscalingmaxreplicas = "5"

  activeregions = ["falkenstein-1", "virginia-1"]
  project       = bahriya_project.web.id
  registry      = bahriya_registry.ghcr.handle

  newenvvar = [
    { key = "NODE_ENV", value = "production" },
  ]

  secretsenvvar = [
    { name = "DATABASE_PASSWORD", secret = bahriya_secret.db_password.handle },
  ]

  hostnames = [
    { hostname = "api.example.com" },
  ]

  # Attach project-scoped vault/config items (mounted as files) and a
  # container-scoped network policy (applies to this container's pods only).
  tlsbundles = [
    { handle = bahriya_tls_bundle.edge.handle, mountpath = "/etc/tls" },
  ]

  networkpolicies = [bahriya_network_policy.web_tier.handle]
}
