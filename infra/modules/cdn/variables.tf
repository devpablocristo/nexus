variable "environment" {
  type = string
}

variable "app_domain" {
  type = string
}

variable "certificate_arn" {
  type    = string
  default = ""
}

variable "force_destroy_bucket" {
  type    = bool
  default = false
}
