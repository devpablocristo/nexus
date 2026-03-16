locals {
  secret_keys = toset(nonsensitive(keys(var.secrets)))
}

resource "aws_secretsmanager_secret" "this" {
  for_each = local.secret_keys

  name = "${var.name_prefix}/${each.value}"

  tags = merge(var.tags, {
    Name = "${var.name_prefix}-${replace(each.value, "/", "-")}"
  })
}

resource "aws_secretsmanager_secret_version" "this" {
  for_each = local.secret_keys

  secret_id     = aws_secretsmanager_secret.this[each.value].id
  secret_string = var.secrets[each.value]
}
