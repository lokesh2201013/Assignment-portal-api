variable "sg_id" {
  description = "Sg id for ALB"
  type = string
}

variable "subnets" {
  description = "Subnets for ALB"
  type = list(string)
}

variable "instancegrp" {
  description = "Web instances for ALB"
  type = list(string)
}