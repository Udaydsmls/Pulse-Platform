terraform {
  backend "s3" {
    bucket         = "pulse-platform-tfstate"
    key            = "prod/terraform.tfstate"
    region         = var.aws_region
    dynamodb_table = "pulse-platform-tflock"
    encrypt        = true
  }
}
