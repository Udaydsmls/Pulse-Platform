resource "random_password" "rds" {
  length           = 32
  special          = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

# ─── Secrets Manager ─────────────────────────────────────────────────────────

resource "aws_secretsmanager_secret" "rds_password" {
  name                    = "${var.project}/${var.environment}/rds-password"
  description             = "Master password for the pulse-platform RDS instance"
  recovery_window_in_days = 7

  tags = {
    Name = "${var.project}-rds-password"
  }
}

resource "aws_secretsmanager_secret_version" "rds_password" {
  secret_id = aws_secretsmanager_secret.rds_password.id
  secret_string = jsonencode({
    username = var.rds_username
    password = random_password.rds.result
  })
}

# ─── RDS Subnet Group ────────────────────────────────────────────────────────

resource "aws_db_subnet_group" "main" {
  name        = "${var.project}-rds-subnet-group"
  description = "Subnet group for pulse-platform RDS"
  subnet_ids  = aws_subnet.private[*].id

  tags = {
    Name = "${var.project}-rds-subnet-group"
  }
}

# ─── RDS Security Group ───────────────────────────────────────────────────────

resource "aws_security_group" "rds" {
  name        = "${var.project}-rds-sg"
  description = "Security group for pulse-platform RDS — allows ingress from EKS nodes"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.eks_nodes.id]
    description     = "PostgreSQL from EKS worker nodes"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound traffic"
  }

  tags = {
    Name = "${var.project}-rds-sg"
  }
}

# ─── Parameter Group ─────────────────────────────────────────────────────────

resource "aws_db_parameter_group" "main" {
  name        = "${var.project}-pg16"
  family      = "postgres16"
  description = "Custom parameter group for pulse-platform PostgreSQL 16"

  parameter {
    name  = "log_statement"
    value = "all"
  }

  parameter {
    name  = "log_connections"
    value = "1"
  }

  parameter {
    name  = "log_disconnections"
    value = "1"
  }

  tags = {
    Name = "${var.project}-pg16"
  }
}

# ─── RDS Instance ─────────────────────────────────────────────────────────────

resource "aws_db_instance" "main" {
  identifier = "${var.project}-${var.environment}"

  engine               = "postgres"
  engine_version       = "16.1"
  instance_class       = var.rds_instance_class
  allocated_storage    = 100
  max_allocated_storage = 500
  storage_type         = "gp3"
  storage_encrypted    = true

  db_name  = var.rds_db_name
  username = var.rds_username
  password = random_password.rds.result

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  parameter_group_name   = aws_db_parameter_group.main.name

  multi_az               = true
  publicly_accessible    = false
  deletion_protection    = true
  skip_final_snapshot    = false
  final_snapshot_identifier = "${var.project}-${var.environment}-final-snapshot"

  backup_retention_period = 7
  backup_window           = "03:00-04:00"
  maintenance_window      = "Mon:04:00-Mon:05:00"

  performance_insights_enabled          = true
  performance_insights_retention_period = 7

  enabled_cloudwatch_logs_exports = ["postgresql", "upgrade"]

  tags = {
    Name = "${var.project}-rds"
  }

  depends_on = [aws_secretsmanager_secret_version.rds_password]
}
