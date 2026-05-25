locals {
  services = [
    "user-service",
    "order-service",
    "inventory-service",
    "payment-service",
    "notification-service",
    "api-gateway",
  ]
}

# ─── ECR Repositories ────────────────────────────────────────────────────────

resource "aws_ecr_repository" "services" {
  for_each = toset(local.services)

  name                 = "${var.project}/${each.key}"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  encryption_configuration {
    encryption_type = "AES256"
  }

  tags = {
    Name = "${var.project}-${each.key}"
  }
}

resource "aws_ecr_lifecycle_policy" "services" {
  for_each   = aws_ecr_repository.services
  repository = each.value.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep last 10 tagged images"
        selection = {
          tagStatus     = "tagged"
          tagPrefixList = ["v"]
          countType     = "imageCountMoreThan"
          countNumber   = 10
        }
        action = { type = "expire" }
      },
      {
        rulePriority = 2
        description  = "Expire untagged images older than 7 days"
        selection = {
          tagStatus   = "untagged"
          countType   = "sinceImagePushed"
          countUnit   = "days"
          countNumber = 7
        }
        action = { type = "expire" }
      },
    ]
  })
}

# ─── Outputs ──────────────────────────────────────────────────────────────────

output "vpc_id" {
  description = "ID of the VPC"
  value       = aws_vpc.main.id
}

output "eks_cluster_name" {
  description = "Name of the EKS cluster"
  value       = aws_eks_cluster.main.name
}

output "eks_cluster_endpoint" {
  description = "API server endpoint of the EKS cluster"
  value       = aws_eks_cluster.main.endpoint
}

output "rds_endpoint" {
  description = "Connection endpoint for the RDS instance"
  value       = aws_db_instance.main.endpoint
  sensitive   = true
}

output "msk_bootstrap_brokers_tls" {
  description = "TLS bootstrap broker string for the MSK cluster"
  value       = aws_msk_cluster.main.bootstrap_brokers_tls
  sensitive   = true
}

output "elasticache_primary_endpoint" {
  description = "Primary endpoint for the ElastiCache Redis replication group"
  value       = aws_elasticache_replication_group.main.primary_endpoint_address
  sensitive   = true
}

output "ecr_repository_urls" {
  description = "Map of service name to ECR repository URL"
  value       = { for k, v in aws_ecr_repository.services : k => v.repository_url }
}
