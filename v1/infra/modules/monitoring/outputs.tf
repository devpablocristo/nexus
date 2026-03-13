output "log_group_names" {
  value = {
    for name, lg in aws_cloudwatch_log_group.services :
    name => lg.name
  }
}

output "sns_topic_arn" {
  value = aws_sns_topic.alerts.arn
}

output "dashboard_name" {
  value = aws_cloudwatch_dashboard.nexus.dashboard_name
}
