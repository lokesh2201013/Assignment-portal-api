variable "vpc_cidr" {
  description = "VPC cidr range"
  type = string
}

variable "subnet_cidr" {
  description = "Subnets CIDRS"
  type = list(string)
}
variable "subnet_names" {
  description = "subnet names"
  type = list(string)
  default = [ "public-subnet-1","public-subnet-2" ]
}

variable "project_id" {
  type = string
  default = <ADD YOUR OWN>
}

