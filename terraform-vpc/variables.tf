variable "vpc_cidr" {
  description = "VPC cidr range"
  type = string
}

variable "subnet_cidr" {
  description = "Subnets CIDRS"
  type = list(string)
}
variable "project_id" {
  type = string
  default = <ADD your  own>
}