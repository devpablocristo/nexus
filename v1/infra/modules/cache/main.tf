locals {
  has_auth_token = trimspace(var.redis_auth_token) != ""
}

resource "aws_elasticache_subnet_group" "main" {
  name       = "${var.environment}-nexus-redis-subnets"
  subnet_ids = var.private_subnet_ids
}

resource "aws_elasticache_replication_group" "redis" {
  replication_group_id = "${var.environment}-nexus-redis"
  description          = "Nexus Redis for rate-limiting"

  engine               = "redis"
  engine_version       = "7.0"
  node_type            = var.redis_node_type
  parameter_group_name = "default.redis7"
  port                 = 6379

  num_node_groups            = 1
  replicas_per_node_group    = var.multi_az_enabled ? 1 : 0
  automatic_failover_enabled = var.multi_az_enabled
  multi_az_enabled           = var.multi_az_enabled

  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token                 = local.has_auth_token ? var.redis_auth_token : null

  subnet_group_name  = aws_elasticache_subnet_group.main.name
  security_group_ids = [var.sg_redis_id]

  maintenance_window = "sun:04:00-sun:05:00"

  tags = {
    Name = "${var.environment}-nexus-redis"
  }
}
