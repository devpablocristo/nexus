variable "environment" {
  type = string
}

variable "private_subnet_ids" {
  type = list(string)
}

variable "sg_redis_id" {
  type = string
}

variable "redis_node_type" {
  type    = string
  default = "cache.t4g.micro"
}

variable "redis_auth_token" {
  type      = string
  sensitive = true
  default   = ""
}

variable "multi_az_enabled" {
  type    = bool
  default = false
}
