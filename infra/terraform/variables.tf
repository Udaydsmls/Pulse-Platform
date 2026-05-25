variable "aws_region" {
  description = "AWS region to deploy resources"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Deployment environment"
  type        = string
  default     = "prod"
}

variable "project" {
  description = "Project name used for resource naming and tagging"
  type        = string
  default     = "pulse-platform"
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "eks_cluster_version" {
  description = "Kubernetes version for the EKS cluster"
  type        = string
  default     = "1.30"
}

variable "eks_node_instance_type" {
  description = "EC2 instance type for EKS worker nodes"
  type        = string
  default     = "t3.medium"
}

variable "eks_node_min" {
  description = "Minimum number of EKS worker nodes"
  type        = number
  default     = 2
}

variable "eks_node_max" {
  description = "Maximum number of EKS worker nodes"
  type        = number
  default     = 6
}

variable "eks_node_desired" {
  description = "Desired number of EKS worker nodes"
  type        = number
  default     = 3
}

variable "rds_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.medium"
}

variable "rds_db_name" {
  description = "Name of the RDS database"
  type        = string
  default     = "pulsedb"
}

variable "rds_username" {
  description = "Master username for the RDS instance"
  type        = string
  default     = "pulseadmin"
}

variable "msk_instance_type" {
  description = "Instance type for MSK broker nodes"
  type        = string
  default     = "kafka.m5.large"
}

variable "elasticache_node_type" {
  description = "ElastiCache node type"
  type        = string
  default     = "cache.t3.medium"
}
