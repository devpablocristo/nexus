output "core_db_endpoint" {
  value = aws_db_instance.nexus_core.address
}

output "saas_db_endpoint" {
  value = aws_db_instance.nexus_saas.address
}

output "core_db_identifier" {
  value = aws_db_instance.nexus_core.identifier
}

output "saas_db_identifier" {
  value = aws_db_instance.nexus_saas.identifier
}

output "instance_identifiers" {
  value = {
    nexus_core = aws_db_instance.nexus_core.identifier
    nexus_saas = aws_db_instance.nexus_saas.identifier
  }
}
