variable "project_name" {
  description = "Project prefix used in AWS resource names."
  type        = string
  default     = "nexus"
}

variable "environment" {
  description = "Deployment environment."
  type        = string

  validation {
    condition     = contains(["staging", "production"], var.environment)
    error_message = "environment must be one of: staging, production."
  }
}

variable "aws_region" {
  description = "AWS region for the deployment."
  type        = string
  default     = "us-east-1"
}

variable "availability_zones" {
  description = "Availability zones used by the VPC."
  type        = list(string)
  default     = ["us-east-1a", "us-east-1b"]
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC."
  type        = string
  default     = "10.42.0.0/16"
}

variable "public_subnet_cidrs" {
  description = "CIDR blocks for public subnets."
  type        = list(string)
  default     = ["10.42.0.0/24", "10.42.1.0/24"]

  validation {
    condition     = length(var.public_subnet_cidrs) == length(var.availability_zones)
    error_message = "public_subnet_cidrs must have the same length as availability_zones."
  }
}

variable "private_subnet_cidrs" {
  description = "CIDR blocks for private subnets."
  type        = list(string)
  default     = ["10.42.10.0/24", "10.42.11.0/24"]

  validation {
    condition     = length(var.private_subnet_cidrs) == length(var.availability_zones)
    error_message = "private_subnet_cidrs must have the same length as availability_zones."
  }
}

variable "allowed_ingress_cidrs" {
  description = "CIDR ranges allowed to reach the public ALB."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "enable_nat_gateway" {
  description = "Whether to provision NAT gateways for private subnets."
  type        = bool
  default     = true
}

variable "single_nat_gateway" {
  description = "Whether to use a single NAT gateway shared across private subnets."
  type        = bool
  default     = true
}

variable "service_discovery_namespace" {
  description = "Private DNS namespace used for ECS service discovery."
  type        = string
  default     = "nexus.internal"
}

variable "enable_https" {
  description = "Whether the public ALB listeners should terminate TLS."
  type        = bool
  default     = false
}

variable "acm_certificate_arn" {
  description = "ACM certificate ARN used when HTTPS is enabled."
  type        = string
  default     = ""

  validation {
    condition     = !var.enable_https || trimspace(var.acm_certificate_arn) != ""
    error_message = "acm_certificate_arn is required when enable_https is true."
  }
}

variable "data_plane_listener_port" {
  description = "Public ALB listener port for data-plane."
  type        = number
  default     = 80
}

variable "control_plane_listener_port" {
  description = "Public ALB listener port for control-plane."
  type        = number
  default     = 8081
}

variable "service_cpu" {
  description = "CPU units per ECS service."
  type        = map(number)
  default = {
    data-plane      = 512
    control-plane   = 512
    control-workers = 512
  }
}

variable "service_memory" {
  description = "Memory reservation per ECS service."
  type        = map(number)
  default = {
    data-plane      = 1024
    control-plane   = 1024
    control-workers = 1024
  }
}

variable "service_desired_count" {
  description = "Desired ECS task count per service."
  type        = map(number)
  default = {
    data-plane      = 2
    control-plane   = 2
    control-workers = 2
  }
}

variable "image_tag" {
  description = "Default image tag pushed to ECR repositories."
  type        = string
  default     = "latest"
}

variable "service_images" {
  description = "Optional full image overrides keyed by service name."
  type        = map(string)
  default     = {}
}

variable "database_name" {
  description = "Logical PostgreSQL database name used by the services."
  type        = string
  default     = "nexus"
}

variable "database_username" {
  description = "Master username for the RDS instance."
  type        = string
  default     = "nexus"
}

variable "database_instance_class" {
  description = "RDS instance class."
  type        = string
  default     = "db.t4g.micro"
}

variable "database_allocated_storage" {
  description = "Initial allocated storage for RDS in GiB."
  type        = number
  default     = 20
}

variable "database_max_allocated_storage" {
  description = "Maximum autoscaled storage for RDS in GiB."
  type        = number
  default     = 100
}

variable "database_engine_version" {
  description = "PostgreSQL engine version."
  type        = string
  default     = "16.4"
}

variable "database_backup_retention_days" {
  description = "Number of days to keep automated backups."
  type        = number
  default     = 7
}

variable "database_multi_az" {
  description = "Whether the RDS instance should use Multi-AZ."
  type        = bool
  default     = false
}

variable "database_publicly_accessible" {
  description = "Whether the RDS instance should be publicly accessible."
  type        = bool
  default     = false
}

variable "database_deletion_protection" {
  description = "Whether the RDS instance should enable deletion protection."
  type        = bool
  default     = false
}

variable "log_retention_in_days" {
  description = "CloudWatch log retention for ECS services."
  type        = number
  default     = 30
}

variable "admin_api_key" {
  description = "Optional admin API key override. When empty, Terraform generates one and stores it in Secrets Manager."
  type        = string
  default     = ""
  sensitive   = true
}

variable "data_plane_service_api_key" {
  description = "Optional data-plane service API key override. When empty, Terraform generates one and stores it in Secrets Manager."
  type        = string
  default     = ""
  sensitive   = true
}

variable "control_workers_service_api_key" {
  description = "Optional control-workers service API key override. When empty, Terraform generates one and stores it in Secrets Manager."
  type        = string
  default     = ""
  sensitive   = true
}

variable "prometheus_api_key" {
  description = "Optional Prometheus API key override. When empty, Terraform generates one and stores it in Secrets Manager."
  type        = string
  default     = ""
  sensitive   = true
}

variable "tags" {
  description = "Additional resource tags."
  type        = map(string)
  default     = {}
}
