environment                    = "production"
aws_region                     = "us-east-1"
availability_zones             = ["us-east-1a", "us-east-1b"]
allowed_ingress_cidrs          = ["0.0.0.0/0"]
enable_https                   = true
acm_certificate_arn            = "arn:aws:acm:us-east-1:123456789012:certificate/replace-me"
data_plane_listener_port       = 443
control_plane_listener_port    = 8443
database_instance_class        = "db.t4g.small"
database_backup_retention_days = 14
database_multi_az              = true
database_deletion_protection   = true

tags = {
  Tier = "production"
}
