variable "aws_region" {
  type        = string
  description = "AWS region for all infrastructure resources"
  default     = "us-east-1"
}

variable "aws_skip_credentials_validation" {
  type        = bool
  description = "Skip AWS credentials validation (useful for local dry plans)"
  default     = false
}

variable "aws_skip_requesting_account_id" {
  type        = bool
  description = "Skip account ID lookup in AWS provider (useful for local dry plans)"
  default     = false
}

variable "aws_skip_metadata_api_check" {
  type        = bool
  description = "Skip EC2 metadata API check in AWS provider"
  default     = false
}

variable "aws_skip_region_validation" {
  type        = bool
  description = "Skip static region validation in AWS provider"
  default     = false
}

variable "environment" {
  type        = string
  description = "Deployment environment: staging or production"

  validation {
    condition     = contains(["staging", "production"], var.environment)
    error_message = "environment must be one of: staging, production."
  }
}

variable "domain" {
  type        = string
  description = "Base domain, e.g. nexus.example.com"
}

variable "vpc_cidr" {
  type        = string
  description = "CIDR block for VPC"
  default     = "10.0.0.0/16"
}

variable "azs" {
  type        = list(string)
  description = "Availability zones used by subnets"
  default     = ["us-east-1a", "us-east-1b"]
}

variable "single_nat_gateway" {
  type        = bool
  description = "When true, creates one shared NAT gateway to reduce cost"
  default     = true
}

variable "certificate_arn" {
  type        = string
  description = "ACM certificate ARN for ALB/CloudFront HTTPS"
  default     = ""
}

variable "create_route53_zone" {
  type        = bool
  description = "Create Route53 hosted zone for var.domain"
  default     = false
}

variable "existing_route53_zone_id" {
  type        = string
  description = "Existing Route53 hosted zone ID when create_route53_zone=false"
  default     = ""
}

variable "core_db_instance_class" {
  type        = string
  description = "RDS instance class for nexus-core database"
  default     = "db.t4g.micro"
}

variable "saas_db_instance_class" {
  type        = string
  description = "RDS instance class for nexus-saas database"
  default     = "db.t4g.micro"
}

variable "core_db_password" {
  type        = string
  description = "Database password for nexus-core PostgreSQL user"
  sensitive   = true
}

variable "saas_db_password" {
  type        = string
  description = "Database password for nexus-saas PostgreSQL user"
  sensitive   = true
}

variable "redis_node_type" {
  type        = string
  description = "ElastiCache node type"
  default     = "cache.t4g.micro"
}

variable "redis_auth_token" {
  type        = string
  description = "Optional Redis AUTH token for in-transit encryption"
  sensitive   = true
  default     = ""
}

variable "core_desired_count" {
  type        = number
  description = "Desired ECS task count for nexus-core"
  default     = 1
}

variable "saas_desired_count" {
  type        = number
  description = "Desired ECS task count for nexus-saas"
  default     = 1
}

variable "control_desired_count" {
  type        = number
  description = "Desired ECS task count for nexus-control-operators"
  default     = 1
}

variable "ai_desired_count" {
  type        = number
  description = "Desired ECS task count for nexus-ai-operators"
  default     = 1
}

variable "image_tag" {
  type        = string
  description = "Container image tag used by ECS task definitions"
  default     = "latest"
}

variable "tower_force_destroy" {
  type        = bool
  description = "When true allows deleting non-empty S3 bucket (dangerous for production)"
  default     = false
}

variable "alert_email" {
  type        = string
  description = "Email subscribed to SNS alarms (optional)"
  default     = ""
}

variable "db_backup_retention_days" {
  type        = number
  description = "Automated RDS backup retention in days"
  default     = 7
}
