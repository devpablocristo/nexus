variable "environment" {
  type = string
}

variable "private_subnet_ids" {
  type = list(string)
}

variable "sg_rds_id" {
  type = string
}

variable "core_db_instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "saas_db_instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "core_db_password" {
  type      = string
  sensitive = true
}

variable "saas_db_password" {
  type      = string
  sensitive = true
}

variable "backup_retention_period" {
  type    = number
  default = 7
}

variable "multi_az_enabled" {
  type    = bool
  default = false
}
