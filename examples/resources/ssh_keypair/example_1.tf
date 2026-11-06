resource "bahriya_ssh_keypair" "deploy" {
  handle     = "deploy"
  name       = "Deploy Key"
  publickey  = file("${path.module}/deploy.pub")
  privatekey = file("${path.module}/deploy")
}
