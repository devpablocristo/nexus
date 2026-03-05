terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  backend "s3" {
    bucket         = "nexus-terraform-state"
    key            = "nexus/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "nexus-terraform-locks"
    encrypt        = true
  }
}

provider "aws" {
  region                      = var.aws_region
  skip_credentials_validation = var.aws_skip_credentials_validation
  skip_requesting_account_id  = var.aws_skip_requesting_account_id
  skip_metadata_api_check     = var.aws_skip_metadata_api_check
  skip_region_validation      = var.aws_skip_region_validation

  default_tags {
    tags = {
      Project     = "nexus"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}

locals {
  container_services = [
    "nexus-core",
    "nexus-saas",
    "nexus-control-operators",
    "nexus-ai-operators",
  ]

  ecs_cluster_name = "${var.environment}-nexus"

  ecs_service_names = {
    "nexus-core"              = "${var.environment}-nexus-core"
    "nexus-saas"              = "${var.environment}-nexus-saas"
    "nexus-control-operators" = "${var.environment}-nexus-control-operators"
    "nexus-ai-operators"      = "${var.environment}-nexus-ai-operators"
  }

  log_group_names = {
    for svc, service_name in local.ecs_service_names :
    svc => "/ecs/${service_name}"
  }

  image_uris = {
    for svc in local.container_services :
    svc => "${module.ecr.repository_urls[svc]}:${var.image_tag}"
  }

  desired_counts = {
    "nexus-core"              = var.core_desired_count
    "nexus-saas"              = var.saas_desired_count
    "nexus-control-operators" = var.control_desired_count
    "nexus-ai-operators"      = var.ai_desired_count
  }

  api_domain = "api.${var.domain}"
  app_domain = "app.${var.domain}"

  dns_zone_id = var.create_route53_zone ? module.dns.zone_id : var.existing_route53_zone_id

  secret_names = [
    "nexus-core-db-password",
    "nexus-saas-db-password",
    "nexus-master-key",
    "nexus-saas-internal-key",
    "nexus-operator-api-key",
    "clerk-secret-key",
    "clerk-webhook-secret",
    "stripe-secret-key",
    "stripe-webhook-secret",
    "anthropic-api-key",
  ]
}

module "networking" {
  source = "./modules/networking"

  environment        = var.environment
  vpc_cidr           = var.vpc_cidr
  azs                = var.azs
  single_nat_gateway = var.single_nat_gateway
}

module "database" {
  source = "./modules/database"

  environment             = var.environment
  private_subnet_ids      = module.networking.private_subnet_ids
  sg_rds_id               = module.networking.sg_rds_id
  core_db_instance_class  = var.core_db_instance_class
  saas_db_instance_class  = var.saas_db_instance_class
  core_db_password        = var.core_db_password
  saas_db_password        = var.saas_db_password
  backup_retention_period = var.db_backup_retention_days
  multi_az_enabled        = var.environment == "production"
}

module "cache" {
  source = "./modules/cache"

  environment        = var.environment
  private_subnet_ids = module.networking.private_subnet_ids
  sg_redis_id        = module.networking.sg_redis_id
  redis_node_type    = var.redis_node_type
  redis_auth_token   = var.redis_auth_token
  multi_az_enabled   = var.environment == "production"
}

module "ecr" {
  source = "./modules/ecr"

  environment = var.environment
  services    = local.container_services
}

module "secrets" {
  source = "./modules/secrets"

  environment  = var.environment
  secret_names = local.secret_names
}

module "loadbalancer" {
  source = "./modules/loadbalancer"

  environment       = var.environment
  vpc_id            = module.networking.vpc_id
  public_subnet_ids = module.networking.public_subnet_ids
  sg_alb_id         = module.networking.sg_alb_id
  certificate_arn   = var.certificate_arn
}

module "monitoring" {
  source = "./modules/monitoring"

  environment               = var.environment
  ecs_cluster_name          = local.ecs_cluster_name
  ecs_service_names         = local.ecs_service_names
  log_group_names           = local.log_group_names
  rds_instance_identifiers  = module.database.instance_identifiers
  alb_arn_suffix            = module.loadbalancer.alb_arn_suffix
  target_group_arn_suffixes = module.loadbalancer.target_group_arn_suffixes
  alarm_email               = var.alert_email
}

module "ecs" {
  source = "./modules/ecs"

  aws_region                  = var.aws_region
  environment                 = var.environment
  cluster_name                = local.ecs_cluster_name
  vpc_id                      = module.networking.vpc_id
  private_subnet_ids          = module.networking.private_subnet_ids
  sg_ecs_id                   = module.networking.sg_ecs_id
  image_uris                  = local.image_uris
  desired_counts              = local.desired_counts
  service_names               = local.ecs_service_names
  log_group_names             = module.monitoring.log_group_names
  core_db_endpoint            = module.database.core_db_endpoint
  saas_db_endpoint            = module.database.saas_db_endpoint
  redis_primary_endpoint      = module.cache.primary_endpoint
  secret_arns                 = module.secrets.secret_arns
  target_group_arns           = module.loadbalancer.target_group_arns
  tower_base_url              = "https://${local.app_domain}"
  service_discovery_namespace = "nexus.local"
}

module "cdn" {
  source = "./modules/cdn"

  environment          = var.environment
  app_domain           = local.app_domain
  certificate_arn      = var.certificate_arn
  force_destroy_bucket = var.tower_force_destroy
}

module "dns" {
  source = "./modules/dns"

  environment        = var.environment
  domain             = var.domain
  create_hosted_zone = var.create_route53_zone
  existing_zone_id   = var.existing_route53_zone_id
  api_domain         = local.api_domain
  app_domain         = local.app_domain
  alb_dns_name       = module.loadbalancer.alb_dns_name
  alb_zone_id        = module.loadbalancer.alb_zone_id
  cloudfront_domain  = module.cdn.cloudfront_domain_name
  cloudfront_zone_id = module.cdn.cloudfront_hosted_zone_id
}
