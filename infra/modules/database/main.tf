resource "aws_db_subnet_group" "main" {
  name       = "${var.environment}-nexus-db-subnets"
  subnet_ids = var.private_subnet_ids

  tags = {
    Name = "${var.environment}-nexus-db-subnets"
  }
}

resource "aws_db_instance" "nexus_core" {
  identifier     = "${var.environment}-nexus-core"
  engine         = "postgres"
  engine_version = "16.4"
  instance_class = var.core_db_instance_class

  db_name  = "nexus"
  username = "nexus_core"
  password = var.core_db_password

  allocated_storage     = 20
  max_allocated_storage = 100
  storage_type          = "gp3"
  storage_encrypted     = true

  backup_retention_period      = var.backup_retention_period
  backup_window                = "03:00-04:00"
  maintenance_window           = "Mon:04:00-Mon:05:00"
  copy_tags_to_snapshot        = true
  deletion_protection          = true
  skip_final_snapshot          = false
  final_snapshot_identifier    = "${var.environment}-nexus-core-final"
  multi_az                     = var.multi_az_enabled
  publicly_accessible          = false
  db_subnet_group_name         = aws_db_subnet_group.main.name
  vpc_security_group_ids       = [var.sg_rds_id]
  auto_minor_version_upgrade   = true
  allow_major_version_upgrade  = false
  performance_insights_enabled = true

  tags = {
    Name = "${var.environment}-nexus-core"
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_db_instance" "nexus_saas" {
  identifier     = "${var.environment}-nexus-saas"
  engine         = "postgres"
  engine_version = "16.4"
  instance_class = var.saas_db_instance_class

  db_name  = "nexus_saas"
  username = "nexus_saas"
  password = var.saas_db_password

  allocated_storage     = 20
  max_allocated_storage = 100
  storage_type          = "gp3"
  storage_encrypted     = true

  backup_retention_period      = var.backup_retention_period
  backup_window                = "03:00-04:00"
  maintenance_window           = "Mon:04:00-Mon:05:00"
  copy_tags_to_snapshot        = true
  deletion_protection          = true
  skip_final_snapshot          = false
  final_snapshot_identifier    = "${var.environment}-nexus-saas-final"
  multi_az                     = var.multi_az_enabled
  publicly_accessible          = false
  db_subnet_group_name         = aws_db_subnet_group.main.name
  vpc_security_group_ids       = [var.sg_rds_id]
  auto_minor_version_upgrade   = true
  allow_major_version_upgrade  = false
  performance_insights_enabled = true

  tags = {
    Name = "${var.environment}-nexus-saas"
  }

  lifecycle {
    prevent_destroy = true
  }
}
