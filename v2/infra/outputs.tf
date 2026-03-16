output "vpc_id" {
  description = "VPC ID used by Nexus v2."
  value       = module.networking.vpc_id
}

output "ecr_repository_urls" {
  description = "ECR repository URLs keyed by service name."
  value       = module.ecr.repository_urls
}

output "alb_dns_name" {
  description = "Public DNS name of the shared ALB."
  value       = module.loadbalancer.dns_name
}

output "public_service_endpoints" {
  description = "Public entrypoints exposed by the ALB."
  value = {
    data-plane    = "${lower(local.public_services["data-plane"].listener_protocol) == "https" ? "https" : "http"}://${module.loadbalancer.dns_name}:${local.public_services["data-plane"].listener_port}"
    control-plane = "${lower(local.public_services["control-plane"].listener_protocol) == "https" ? "https" : "http"}://${module.loadbalancer.dns_name}:${local.public_services["control-plane"].listener_port}"
  }
}

output "internal_service_endpoints" {
  description = "Private DNS endpoints used for service-to-service traffic."
  value       = local.internal_service_urls
}

output "database_endpoint" {
  description = "RDS endpoint for the shared PostgreSQL instance."
  value       = module.database.endpoint
}

output "database_secret_arn" {
  description = "Secrets Manager ARN containing the RDS master password."
  value       = module.database.master_secret_arn
}

output "runtime_secret_arns" {
  description = "Secrets Manager ARNs used by ECS tasks."
  value       = module.secrets.secret_arns
  sensitive   = true
}

output "api_key_secret_arns" {
  description = "Secrets Manager ARNs containing the generated or overridden API keys."
  value = {
    admin                   = module.secrets.secret_arns["api-keys/admin"]
    data_plane_service      = module.secrets.secret_arns["api-keys/data-plane-service"]
    control_workers_service = module.secrets.secret_arns["api-keys/control-workers-service"]
    prometheus              = module.secrets.secret_arns["api-keys/prometheus"]
  }
  sensitive = true
}
