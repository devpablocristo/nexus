variable "environment" {
  type = string
}

variable "domain" {
  type = string
}

variable "create_hosted_zone" {
  type    = bool
  default = false
}

variable "existing_zone_id" {
  type    = string
  default = ""
}

variable "api_domain" {
  type = string
}

variable "app_domain" {
  type = string
}

variable "alb_dns_name" {
  type = string
}

variable "alb_zone_id" {
  type = string
}

variable "cloudfront_domain" {
  type = string
}

variable "cloudfront_zone_id" {
  type = string
}
