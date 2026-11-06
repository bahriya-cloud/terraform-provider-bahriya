resource "bahriya_gpg_keypair" "signing_key" {
  # ... bahriya_gpg_keypair configuration ...
}

resource "bahriya_project_gpg_keypair_attachment" "web_gpg_keypair" {
  project_id = bahriya_project.web.id
  handle     = bahriya_gpg_keypair.signing_key.handle
}
