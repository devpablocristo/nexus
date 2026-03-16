variable "name_prefix" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "public_subnet_ids" {
  type = list(string)
}

variable "allowed_cidrs" {
  type = list(string)
}

variable "services" {
  type = map(object({
    listener_port     = number
    listener_protocol = string
    target_port       = number
    health_check_path = string
    certificate_arn   = string
  }))
}

variable "tags" {
  type    = map(string)
  default = {}
}

