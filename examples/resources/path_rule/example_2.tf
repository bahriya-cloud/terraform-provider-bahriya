resource "bahriya_path_rule" "webhook" {
  container_id = bahriya_container.api.id
  handle       = "webhook"
  path         = "/webhook"

  ratelimitingenabled           = true
  ratelimitingrequestsperminute = 60
  ratelimitingrequestsperhour   = 1000
}
