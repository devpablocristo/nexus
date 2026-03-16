output "secret_arns" {
  sensitive = true
  value = {
    for name, secret in aws_secretsmanager_secret.this :
    name => secret.arn
  }
}

