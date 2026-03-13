output "alb_dns_name" {
  description = "Public DNS name of the Application Load Balancer"
  value       = module.loadbalancer.alb_dns_name
}

output "cloudfront_domain_name" {
  description = "CloudFront distribution domain for nexus-tower"
  value       = module.cdn.cloudfront_domain_name
}

output "tower_bucket_name" {
  description = "S3 bucket hosting the tower SPA artifacts"
  value       = module.cdn.bucket_name
}

output "api_domain" {
  description = "API public hostname"
  value       = "api.${var.domain}"
}

output "app_domain" {
  description = "App public hostname"
  value       = "app.${var.domain}"
}

output "rds_endpoints" {
  description = "RDS instance endpoints"
  value = {
    nexus_core = module.database.core_db_endpoint
    nexus_saas = module.database.saas_db_endpoint
  }
}

output "redis_primary_endpoint" {
  description = "Primary Redis endpoint"
  value       = module.cache.primary_endpoint
}

output "ecr_repository_urls" {
  description = "ECR repository URLs by service"
  value       = module.ecr.repository_urls
}

output "ecs_cluster_name" {
  description = "ECS cluster name"
  value       = module.ecs.cluster_name
}

output "ecs_service_names" {
  description = "ECS service names by service key"
  value       = module.ecs.service_names
}

output "service_discovery_namespace" {
  description = "Cloud Map namespace for internal service discovery"
  value       = module.ecs.service_discovery_namespace
}

output "secrets_arns" {
  description = "Secrets Manager ARNs"
  value       = module.secrets.secret_arns
  sensitive   = true
}
