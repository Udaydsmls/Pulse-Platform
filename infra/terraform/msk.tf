# ─── MSK Security Group ───────────────────────────────────────────────────────

resource "aws_security_group" "msk" {
  name        = "${var.project}-msk-sg"
  description = "Security group for pulse-platform MSK cluster"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port       = 9094
    to_port         = 9094
    protocol        = "tcp"
    security_groups = [aws_security_group.eks_nodes.id]
    description     = "Kafka TLS from EKS worker nodes"
  }

  ingress {
    from_port       = 2181
    to_port         = 2181
    protocol        = "tcp"
    security_groups = [aws_security_group.eks_nodes.id]
    description     = "ZooKeeper from EKS worker nodes"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound traffic"
  }

  tags = {
    Name = "${var.project}-msk-sg"
  }
}

# ─── MSK Configuration ───────────────────────────────────────────────────────

resource "aws_msk_configuration" "main" {
  name              = "${var.project}-msk-config"
  description       = "MSK configuration for pulse-platform"
  kafka_versions    = ["3.6.0"]

  server_properties = <<-PROPERTIES
    auto.create.topics.enable=false
    default.replication.factor=3
    min.insync.replicas=2
    num.partitions=6
    log.retention.hours=168
    log.segment.bytes=1073741824
    log.retention.check.interval.ms=300000
    offsets.topic.replication.factor=3
    transaction.state.log.min.isr=2
    transaction.state.log.replication.factor=3
  PROPERTIES
}

# ─── MSK Cluster ─────────────────────────────────────────────────────────────

resource "aws_msk_cluster" "main" {
  cluster_name           = "${var.project}-kafka"
  kafka_version          = "3.6.0"
  number_of_broker_nodes = 3

  broker_node_group_info {
    instance_type   = var.msk_instance_type
    client_subnets  = aws_subnet.private[*].id
    security_groups = [aws_security_group.msk.id]

    storage_info {
      ebs_storage_info {
        volume_size = 100
      }
    }
  }

  configuration_info {
    arn      = aws_msk_configuration.main.arn
    revision = aws_msk_configuration.main.latest_revision
  }

  encryption_info {
    encryption_in_transit {
      client_broker = "TLS"
      in_cluster    = true
    }
  }

  client_authentication {
    tls {}
  }

  open_monitoring {
    prometheus {
      jmx_exporter {
        enabled_in_broker = true
      }
      node_exporter {
        enabled_in_broker = true
      }
    }
  }

  logging {
    broker_logs {
      cloudwatch_logs {
        enabled   = true
        log_group = "/aws/msk/${var.project}"
      }
    }
  }

  tags = {
    Name = "${var.project}-kafka"
  }
}
