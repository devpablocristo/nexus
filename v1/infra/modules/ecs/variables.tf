variable "aws_region" {
  type = string
}

variable "environment" {
  type = string
}

variable "cluster_name" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "private_subnet_ids" {
  type = list(string)
}

variable "sg_ecs_id" {
  type = string
}

variable "service_discovery_namespace" {
  type    = string
  default = "nexus.local"
}

variable "service_names" {
  type = map(string)
}

variable "image_uris" {
  type = map(string)
}

variable "desired_counts" {
  type = map(number)
}

variable "target_group_arns" {
  type = map(string)
}

variable "log_group_names" {
  type = map(string)
}

variable "core_db_endpoint" {
  type = string
}

variable "saas_db_endpoint" {
  type = string
}

variable "redis_primary_endpoint" {
  type = string
}

variable "secret_arns" {
  type = map(string)
}

variable "tower_base_url" {
  type = string
}

variable "cpu_map" {
  type = map(number)
  default = {
    "nexus-core"              = 512
    "nexus-saas"              = 512
    "nexus-control-operators" = 256
    "nexus-ai-operators"      = 512
  }
}

variable "memory_map" {
  type = map(number)
  default = {
    "nexus-core"              = 1024
    "nexus-saas"              = 1024
    "nexus-control-operators" = 512
    "nexus-ai-operators"      = 1024
  }
}
