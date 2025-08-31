
module "vpc" {
  source = "./modules/vpc"
  vpc_cidr = var.vpc_cidr
  subnet_cidr = var.subnet_cidr
}

module "sg" {
  source = "./modules/sg"
  network_id=module.vpc.network_id
}

module "ec2" {
  source = "./modules/ec2"
  instance_tags = module.sg.instance_tag
  subnets = module.vpc.subnets_id
}

module "alb" {
  source = "./modules/alb"
  sg_id = module.sg.sg_id
  subnets = module.vpc.subnets_id
   instancegrp = module.ec2.instance[*]
}