resource "bahriya_ssh_keypair" "deploy_key" {
  # ... bahriya_ssh_keypair configuration ...
}

resource "bahriya_project_ssh_keypair_attachment" "web_ssh_keypair" {
  project_id = bahriya_project.web.id
  handle     = bahriya_ssh_keypair.deploy_key.handle
}
