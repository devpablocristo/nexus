output "cluster_arn" {
  value = aws_ecs_cluster.this.arn
}

output "service_names" {
  value = {
    for name, service in aws_ecs_service.this :
    name => service.name
  }
}

output "service_discovery_namespace_id" {
  value = aws_service_discovery_private_dns_namespace.this.id
}
