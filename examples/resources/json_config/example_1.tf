resource "bahriya_json_config" "app" {
  handle = "app-json"
  name   = "App JSON"
  content = jsonencode({
    server = {
      port    = 8080
      timeout = "30s"
    }
    features = {
      beta = false
    }
  })
}
