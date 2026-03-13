output "secret_arns" {
  value = {
    for name, secret in aws_secretsmanager_secret.items :
    name => secret.arn
  }
}

output "secret_names" {
  value = {
    for name, secret in aws_secretsmanager_secret.items :
    name => secret.name
  }
}
