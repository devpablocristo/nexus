variable "name_prefix" {
  type = string
}

variable "secrets" {
  type      = map(string)
  sensitive = true
}

variable "tags" {
  type    = map(string)
  default = {}
}

