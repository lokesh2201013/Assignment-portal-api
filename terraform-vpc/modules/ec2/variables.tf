variable "instance_tags" {
  description = "tags to apply to instance"
  type = list(string)
}

variable "subnets" {
  description = "subnets for vm instances"
  type = list(string)
}

variable "ec2_names" {
  description = "ec2 names"
  type = list(string)
  default = [ "w1", "w2" ]
}