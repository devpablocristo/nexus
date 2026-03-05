resource "aws_secretsmanager_secret" "items" {
  for_each = toset(var.secret_names)

  name                    = "${var.environment}/${each.value}"
  recovery_window_in_days = 7

  tags = {
    Name = "${var.environment}-${each.value}"
  }
}
