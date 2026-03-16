variable "name_prefix" {
  type = string
}

variable "repositories" {
  type = set(string)
}

variable "tags" {
  type    = map(string)
  default = {}
}

