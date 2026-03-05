locals {
  zone_id = var.create_hosted_zone ? aws_route53_zone.main[0].zone_id : trimspace(var.existing_zone_id)
}

resource "aws_route53_zone" "main" {
  count = var.create_hosted_zone ? 1 : 0
  name  = var.domain

  tags = {
    Name = "${var.environment}-nexus-zone"
  }
}

resource "aws_route53_record" "api" {
  count   = local.zone_id != "" ? 1 : 0
  zone_id = local.zone_id
  name    = var.api_domain
  type    = "A"

  alias {
    name                   = var.alb_dns_name
    zone_id                = var.alb_zone_id
    evaluate_target_health = true
  }
}

resource "aws_route53_record" "app" {
  count   = local.zone_id != "" ? 1 : 0
  zone_id = local.zone_id
  name    = var.app_domain
  type    = "A"

  alias {
    name                   = var.cloudfront_domain
    zone_id                = var.cloudfront_zone_id
    evaluate_target_health = false
  }
}
