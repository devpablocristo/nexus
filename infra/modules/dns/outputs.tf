output "zone_id" {
  value = local.zone_id
}

output "api_record_fqdn" {
  value = try(aws_route53_record.api[0].fqdn, "")
}

output "app_record_fqdn" {
  value = try(aws_route53_record.app[0].fqdn, "")
}
