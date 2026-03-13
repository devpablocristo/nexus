output "alb_dns_name" {
  value = aws_lb.main.dns_name
}

output "alb_zone_id" {
  value = aws_lb.main.zone_id
}

output "alb_arn" {
  value = aws_lb.main.arn
}

output "alb_arn_suffix" {
  value = aws_lb.main.arn_suffix
}

output "target_group_arns" {
  value = {
    "nexus-core" = aws_lb_target_group.core.arn
    "nexus-saas" = aws_lb_target_group.saas.arn
  }
}

output "target_group_arn_suffixes" {
  value = {
    "nexus-core" = aws_lb_target_group.core.arn_suffix
    "nexus-saas" = aws_lb_target_group.saas.arn_suffix
  }
}
