# ─── ElastiCache Security Group ───────────────────────────────────────────────

resource "aws_security_group" "redis" {
  name        = "${var.project}-redis-sg"
  description = "Security group for pulse-platform ElastiCache Redis"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [aws_security_group.eks_nodes.id]
    description     = "Redis from EKS worker nodes"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound traffic"
  }

  tags = {
    Name = "${var.project}-redis-sg"
  }
}

# ─── ElastiCache Subnet Group ────────────────────────────────────────────────

resource "aws_elasticache_subnet_group" "main" {
  name        = "${var.project}-redis-subnet-group"
  description = "Subnet group for pulse-platform ElastiCache Redis"
  subnet_ids  = aws_subnet.private[*].id

  tags = {
    Name = "${var.project}-redis-subnet-group"
  }
}

# ─── ElastiCache Replication Group ───────────────────────────────────────────

resource "aws_elasticache_replication_group" "main" {
  replication_group_id = "${var.project}-redis"
  description          = "Redis replication group for pulse-platform"

  engine               = "redis"
  engine_version       = "7.2"
  node_type            = var.elasticache_node_type
  port                 = 6379

  num_cache_clusters         = 2
  automatic_failover_enabled = true
  multi_az_enabled           = true

  subnet_group_name  = aws_elasticache_subnet_group.main.name
  security_group_ids = [aws_security_group.redis.id]

  at_rest_encryption_enabled  = true
  transit_encryption_enabled  = true
  transit_encryption_mode     = "required"

  maintenance_window       = "sun:05:00-sun:06:00"
  snapshot_window          = "03:00-04:00"
  snapshot_retention_limit = 7

  auto_minor_version_upgrade = true

  log_delivery_configuration {
    destination      = "/aws/elasticache/${var.project}/redis-slow-logs"
    destination_type = "cloudwatch-logs"
    log_format       = "json"
    log_type         = "slow-log"
  }

  log_delivery_configuration {
    destination      = "/aws/elasticache/${var.project}/redis-engine-logs"
    destination_type = "cloudwatch-logs"
    log_format       = "json"
    log_type         = "engine-log"
  }

  tags = {
    Name = "${var.project}-redis"
  }
}
