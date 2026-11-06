resource "bahriya_registry" "ecr" {
  handle   = "ecr"
  name     = "AWS ECR"
  server   = "123456789.dkr.ecr.us-east-1.amazonaws.com"
  username = "AWS"
  password = var.ecr_token
}
