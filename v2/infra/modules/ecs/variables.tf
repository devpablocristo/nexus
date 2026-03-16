variable "name_prefix" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "private_subnet_ids" {
  type = list(string)
}

variable "application_security_group_id" {
  type = string
}

variable "alb_security_group_id" {
  type    = string
  default = null
}

variable "service_discovery_namespace" {
  type = string
}

variable "log_retention_in_days" {
  type = number
}

variable "services" {
  type = map(object({
    image            = string
    container_port   = number
    cpu              = number
    memory           = number
    desired_count    = number
    environment      = map(string)
    secrets          = map(string)
    target_group_arn = optional(string)
  }))
}

variable "tags" {
  type    = map(string)
  default = {}
}

