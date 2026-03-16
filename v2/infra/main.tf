locals {
  name_prefix = "${var.project_name}-${var.environment}"

  service_ports = {
    data-plane      = 8080
    control-plane   = 8081
    control-workers = 8082
  }

  public_listener_protocol = var.enable_https ? "HTTPS" : "HTTP"

  public_services = {
    data-plane = {
      listener_port     = var.data_plane_listener_port
      listener_protocol = local.public_listener_protocol
      target_port       = local.service_ports["data-plane"]
      health_check_path = "/readyz"
      certificate_arn   = trimspace(var.acm_certificate_arn)
    }
    control-plane = {
      listener_port     = var.control_plane_listener_port
      listener_protocol = local.public_listener_protocol
      target_port       = local.service_ports["control-plane"]
      health_check_path = "/readyz"
      certificate_arn   = trimspace(var.acm_certificate_arn)
    }
  }
}

module "networking" {
  source = "./modules/networking"

  name_prefix          = local.name_prefix
  vpc_cidr             = var.vpc_cidr
  azs                  = var.availability_zones
  public_subnet_cidrs  = var.public_subnet_cidrs
  private_subnet_cidrs = var.private_subnet_cidrs
  enable_nat_gateway   = var.enable_nat_gateway
  single_nat_gateway   = var.single_nat_gateway
  tags                 = var.tags
}

module "ecr" {
  source = "./modules/ecr"

  name_prefix = local.name_prefix
  repositories = toset([
    "data-plane",
    "control-plane",
    "control-workers",
  ])
  tags = var.tags
}

module "database" {
  source = "./modules/database"

  name_prefix                = local.name_prefix
  vpc_id                     = module.networking.vpc_id
  subnet_ids                 = module.networking.private_subnet_ids
  application_security_group = module.networking.application_security_group_id
  database_name              = var.database_name
  database_username          = var.database_username
  instance_class             = var.database_instance_class
  allocated_storage          = var.database_allocated_storage
  max_allocated_storage      = var.database_max_allocated_storage
  engine_version             = var.database_engine_version
  backup_retention_days      = var.database_backup_retention_days
  multi_az                   = var.database_multi_az
  publicly_accessible        = var.database_publicly_accessible
  deletion_protection        = var.database_deletion_protection
  tags                       = var.tags
}

locals {
  database_url = format(
    "postgres://%s:%s@%s:%d/%s?sslmode=require",
    urlencode(module.database.master_username),
    urlencode(module.database.master_password),
    module.database.address,
    module.database.port,
    var.database_name,
  )

  requested_api_key_overrides = {
    admin           = trimspace(var.admin_api_key)
    data-plane      = trimspace(var.data_plane_service_api_key)
    control-workers = trimspace(var.control_workers_service_api_key)
    prometheus      = trimspace(var.prometheus_api_key)
  }
}

resource "random_password" "api_key" {
  for_each = toset(["admin", "data-plane", "control-workers", "prometheus"])

  length  = 40
  special = false
}

locals {
  resolved_api_keys = {
    for name, override in local.requested_api_key_overrides :
    name => (
      override != "" ?
      override :
      random_password.api_key[name].result
    )
  }

  secret_values = {
    "api-keys/admin"                        = local.resolved_api_keys["admin"]
    "api-keys/data-plane-service"           = local.resolved_api_keys["data-plane"]
    "api-keys/control-workers-service"      = local.resolved_api_keys["control-workers"]
    "api-keys/prometheus"                   = local.resolved_api_keys["prometheus"]
    "data-plane/nexus_api_keys"             = "admin=${local.resolved_api_keys["admin"]},prometheus=${local.resolved_api_keys["prometheus"]}"
    "data-plane/control_plane_api_key"      = local.resolved_api_keys["data-plane"]
    "data-plane/control_workers_api_key"    = local.resolved_api_keys["data-plane"]
    "data-plane/database_url"               = local.database_url
    "control-plane/nexus_api_keys"          = "admin=${local.resolved_api_keys["admin"]},data-plane=${local.resolved_api_keys["data-plane"]},control-workers=${local.resolved_api_keys["control-workers"]},prometheus=${local.resolved_api_keys["prometheus"]}"
    "control-plane/database_url"            = local.database_url
    "control-plane/audit_database_url"      = local.database_url
    "control-workers/nexus_api_keys"        = "admin=${local.resolved_api_keys["admin"]},data-plane=${local.resolved_api_keys["data-plane"]},prometheus=${local.resolved_api_keys["prometheus"]}"
    "control-workers/control_plane_api_key" = local.resolved_api_keys["control-workers"]
    "control-workers/database_url"          = local.database_url
  }
}

module "secrets" {
  source = "./modules/secrets"

  name_prefix = local.name_prefix
  secrets     = local.secret_values
  tags        = var.tags
}

module "loadbalancer" {
  source = "./modules/loadbalancer"

  name_prefix       = local.name_prefix
  vpc_id            = module.networking.vpc_id
  public_subnet_ids = module.networking.public_subnet_ids
  allowed_cidrs     = var.allowed_ingress_cidrs
  services          = local.public_services
  tags              = var.tags
}

locals {
  default_service_images = {
    for name, repo in module.ecr.repository_urls :
    name => "${repo}:${var.image_tag}"
  }

  service_images = merge(local.default_service_images, var.service_images)

  internal_service_urls = {
    control-plane   = "http://control-plane.${var.service_discovery_namespace}:${local.service_ports["control-plane"]}"
    control-workers = "http://control-workers.${var.service_discovery_namespace}:${local.service_ports["control-workers"]}"
  }

  ecs_services = {
    data-plane = {
      image          = local.service_images["data-plane"]
      container_port = local.service_ports["data-plane"]
      cpu            = lookup(var.service_cpu, "data-plane", 512)
      memory         = lookup(var.service_memory, "data-plane", 1024)
      desired_count  = lookup(var.service_desired_count, "data-plane", 2)
      environment = {
        PORT                      = tostring(local.service_ports["data-plane"])
        NEXUS_CONTROL_PLANE_URL   = local.internal_service_urls["control-plane"]
        NEXUS_CONTROL_WORKERS_URL = local.internal_service_urls["control-workers"]
      }
      secrets = {
        NEXUS_API_KEYS                = module.secrets.secret_arns["data-plane/nexus_api_keys"]
        NEXUS_CONTROL_PLANE_API_KEY   = module.secrets.secret_arns["data-plane/control_plane_api_key"]
        NEXUS_CONTROL_WORKERS_API_KEY = module.secrets.secret_arns["data-plane/control_workers_api_key"]
        NEXUS_DATA_PLANE_DATABASE_URL = module.secrets.secret_arns["data-plane/database_url"]
      }
      target_group_arn = module.loadbalancer.target_group_arns["data-plane"]
    }
    control-plane = {
      image          = local.service_images["control-plane"]
      container_port = local.service_ports["control-plane"]
      cpu            = lookup(var.service_cpu, "control-plane", 512)
      memory         = lookup(var.service_memory, "control-plane", 1024)
      desired_count  = lookup(var.service_desired_count, "control-plane", 2)
      environment = {
        PORT = tostring(local.service_ports["control-plane"])
      }
      secrets = {
        NEXUS_API_KEYS                   = module.secrets.secret_arns["control-plane/nexus_api_keys"]
        NEXUS_CONTROL_PLANE_DATABASE_URL = module.secrets.secret_arns["control-plane/database_url"]
        NEXUS_AUDIT_DATABASE_URL         = module.secrets.secret_arns["control-plane/audit_database_url"]
      }
      target_group_arn = module.loadbalancer.target_group_arns["control-plane"]
    }
    control-workers = {
      image          = local.service_images["control-workers"]
      container_port = local.service_ports["control-workers"]
      cpu            = lookup(var.service_cpu, "control-workers", 512)
      memory         = lookup(var.service_memory, "control-workers", 1024)
      desired_count  = lookup(var.service_desired_count, "control-workers", 2)
      environment = {
        PORT                    = tostring(local.service_ports["control-workers"])
        NEXUS_CONTROL_PLANE_URL = local.internal_service_urls["control-plane"]
      }
      secrets = {
        NEXUS_API_KEYS                     = module.secrets.secret_arns["control-workers/nexus_api_keys"]
        NEXUS_CONTROL_PLANE_API_KEY        = module.secrets.secret_arns["control-workers/control_plane_api_key"]
        NEXUS_CONTROL_WORKERS_DATABASE_URL = module.secrets.secret_arns["control-workers/database_url"]
      }
      target_group_arn = null
    }
  }
}

module "ecs" {
  source = "./modules/ecs"

  name_prefix                   = local.name_prefix
  vpc_id                        = module.networking.vpc_id
  private_subnet_ids            = module.networking.private_subnet_ids
  application_security_group_id = module.networking.application_security_group_id
  alb_security_group_id         = module.loadbalancer.security_group_id
  service_discovery_namespace   = var.service_discovery_namespace
  log_retention_in_days         = var.log_retention_in_days
  services                      = local.ecs_services
  tags                          = var.tags
}
