resource "bahriya_gpg_keypair" "signing" {
  handle     = "release-signing"
  name       = "Release Signing Key"
  publickey  = file("${path.module}/signing.pub.asc")
  privatekey = file("${path.module}/signing.sec.asc")
}
