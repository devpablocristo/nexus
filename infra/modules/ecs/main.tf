locals {
  service_ports = {
    "nexus-core"              = 8080
    "nexus-saas"              = 8082
    "nexus-control-operators" = 8090
    "nexus-ai-operators"      = 8000
  }

  health_paths = {
    "nexus-core"              = "/readyz"
    "nexus-saas"              = "/health"
    "nexus-control-operators" = "/healthz"
    "nexus-ai-operators"      = "/readyz"
  }

  # Runtime env vars by service (non-secret values)
  env_map = {
    "nexus-core" = {
      NEXUS_HTTP_PORT          = "8080"
      NEXUS_REDIS_URL          = "redis://${var.redis_primary_endpoint}:6379/0"
      NEXUS_SAAS_URL           = "http://nexus-saas.${var.service_discovery_namespace}:8082"
      NEXUS_AUTH_ALLOW_API_KEY = "true"
      NEXUS_AUTH_ENABLE_JWT    = "true"
      NEXUS_SWAGGER_CDN        = "true"
    }

    "nexus-saas" = {
      NEXUS_HTTP_PORT          = "8082"
      NEXUS_CORE_URL           = "http://nexus-core.${var.service_discovery_namespace}:8080"
      NEXUS_AUTH_ALLOW_API_KEY = "true"
      NEXUS_AUTH_ENABLE_JWT    = "true"
      NEXUS_SWAGGER_CDN        = "true"
      TOWER_BASE_URL           = var.tower_base_url
    }

    "nexus-control-operators" = {
      NEXUS_CORE_URL            = "http://nexus-core.${var.service_discovery_namespace}:8080"
      OPERATOR_HEALTH_PORT      = "8090"
      OPERATOR_DATA_DIR         = "/app/data"
      OPERATOR_BATCH_SIZE       = "100"
      OPERATOR_POLL_INTERVAL_MS = "700"
      OPERATOR_IDLE_INTERVAL_MS = "15000"
    }

    "nexus-ai-operators" = {
      OPERATOR_PORT                    = "8000"
      OPERATOR_ENV                     = var.environment
      NEXUS_CORE_BASE_URL              = "http://nexus-core.${var.service_discovery_namespace}:8080"
      NEXUS_SAAS_BASE_URL              = "http://nexus-saas.${var.service_discovery_namespace}:8082"
      OPERATOR_POLL_INTERVAL_SECONDS   = "10"
      OPERATOR_POLL_BATCH_SIZE         = "100"
      OPERATOR_DENY_RATIO_THRESHOLD    = "0.35"
      OPERATOR_MIN_EVENTS_FOR_SIGNAL   = "20"
      OPERATOR_ACTION_COOLDOWN_SECONDS = "300"
      OPERATOR_ACTION_TTL_SECONDS      = "300"
    }
  }

  # Secret env vars by service
  secret_env_map = {
    "nexus-core" = {
      CORE_DB_PASSWORD        = try(var.secret_arns["nexus-core-db-password"], null)
      NEXUS_MASTER_KEY        = try(var.secret_arns["nexus-master-key"], null)
      NEXUS_SAAS_INTERNAL_KEY = try(var.secret_arns["nexus-saas-internal-key"], null)
      NEXUS_OPERATOR_API_KEY  = try(var.secret_arns["nexus-operator-api-key"], null)
    }

    "nexus-saas" = {
      SAAS_DB_PASSWORD        = try(var.secret_arns["nexus-saas-db-password"], null)
      NEXUS_MASTER_KEY        = try(var.secret_arns["nexus-master-key"], null)
      NEXUS_SAAS_INTERNAL_KEY = try(var.secret_arns["nexus-saas-internal-key"], null)
      CLERK_SECRET_KEY        = try(var.secret_arns["clerk-secret-key"], null)
      CLERK_WEBHOOK_SECRET    = try(var.secret_arns["clerk-webhook-secret"], null)
      STRIPE_SECRET_KEY       = try(var.secret_arns["stripe-secret-key"], null)
      STRIPE_WEBHOOK_SECRET   = try(var.secret_arns["stripe-webhook-secret"], null)
    }

    "nexus-control-operators" = {
      OPERATOR_INTERNAL_KEY = try(var.secret_arns["nexus-operator-api-key"], null)
    }

    "nexus-ai-operators" = {
      OPERATOR_INTERNAL_KEY = try(var.secret_arns["nexus-operator-api-key"], null)
      ANTHROPIC_API_KEY     = try(var.secret_arns["anthropic-api-key"], null)
    }
  }

  command_map = {
    "nexus-core" = [
      "sh",
      "-c",
      "export NEXUS_DATABASE_URL=\"postgres://nexus_core:$${CORE_DB_PASSWORD}@${var.core_db_endpoint}:5432/nexus?sslmode=require\" && exec /app/nexus-core",
    ]

    "nexus-saas" = [
      "sh",
      "-c",
      "export NEXUS_SAAS_DATABASE_URL=\"postgres://nexus_saas:$${SAAS_DB_PASSWORD}@${var.saas_db_endpoint}:5432/nexus_saas?sslmode=require\" && exec /app/nexus-saas",
    ]

    "nexus-control-operators" = null
    "nexus-ai-operators"      = null
  }

  service_defs = {
    for svc, service_name in var.service_names : svc => {
      service_name = service_name
      port         = local.service_ports[svc]
      health_path  = local.health_paths[svc]
      cpu          = tostring(var.cpu_map[svc])
      memory       = tostring(var.memory_map[svc])
      desired      = var.desired_counts[svc]
      image        = var.image_uris[svc]
      log_group    = var.log_group_names[svc]
      command      = local.command_map[svc]
    }
  }
}

resource "aws_ecs_cluster" "main" {
  name = var.cluster_name

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

resource "aws_iam_role" "task_execution" {
  name = "${var.environment}-nexus-ecs-task-exec-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "task_execution_managed" {
  role       = aws_iam_role.task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role_policy" "task_execution_secrets" {
  name = "${var.environment}-nexus-ecs-task-exec-secrets"
  role = aws_iam_role.task_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue",
          "kms:Decrypt",
        ]
        Resource = values(var.secret_arns)
      }
    ]
  })
}

resource "aws_iam_role" "task" {
  name = "${var.environment}-nexus-ecs-task-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
}

resource "aws_service_discovery_private_dns_namespace" "main" {
  name = var.service_discovery_namespace
  vpc  = var.vpc_id
}

resource "aws_service_discovery_service" "services" {
  for_each = local.service_defs

  name = each.key

  dns_config {
    namespace_id = aws_service_discovery_private_dns_namespace.main.id

    dns_records {
      ttl  = 10
      type = "A"
    }

    routing_policy = "MULTIVALUE"
  }

  health_check_custom_config {
    failure_threshold = 1
  }
}

resource "aws_ecs_task_definition" "services" {
  for_each = local.service_defs

  family                   = each.value.service_name
  cpu                      = each.value.cpu
  memory                   = each.value.memory
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  execution_role_arn       = aws_iam_role.task_execution.arn
  task_role_arn            = aws_iam_role.task.arn

  container_definitions = jsonencode([
    merge(
      {
        name      = each.key
        image     = each.value.image
        essential = true

        portMappings = [
          {
            containerPort = each.value.port
            hostPort      = each.value.port
            protocol      = "tcp"
          }
        ]

        environment = [
          for k, v in local.env_map[each.key] : {
            name  = k
            value = tostring(v)
          }
        ]

        secrets = [
          for name, arn in local.secret_env_map[each.key] : {
            name      = name
            valueFrom = arn
          } if arn != null
        ]

        logConfiguration = {
          logDriver = "awslogs"
          options = {
            awslogs-group         = each.value.log_group
            awslogs-region        = var.aws_region
            awslogs-stream-prefix = each.key
          }
        }

        healthCheck = {
          command     = ["CMD-SHELL", "wget -qO- http://localhost:${each.value.port}${each.value.health_path} >/dev/null 2>&1 || exit 1"]
          interval    = 30
          timeout     = 5
          retries     = 3
          startPeriod = 30
        }
      },
      each.value.command == null ? {} : { command = each.value.command }
    )
  ])
}

resource "aws_ecs_service" "services" {
  for_each = local.service_defs

  name            = each.value.service_name
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.services[each.key].arn
  desired_count   = each.value.desired
  launch_type     = "FARGATE"

  deployment_minimum_healthy_percent = 50
  deployment_maximum_percent         = 200

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [var.sg_ecs_id]
    assign_public_ip = false
  }

  dynamic "load_balancer" {
    for_each = contains(keys(var.target_group_arns), each.key) ? [1] : []

    content {
      target_group_arn = var.target_group_arns[each.key]
      container_name   = each.key
      container_port   = each.value.port
    }
  }

  service_registries {
    registry_arn = aws_service_discovery_service.services[each.key].arn
  }

  depends_on = [
    aws_iam_role_policy_attachment.task_execution_managed,
    aws_iam_role_policy.task_execution_secrets,
  ]
}

resource "aws_appautoscaling_target" "ecs" {
  for_each = aws_ecs_service.services

  max_capacity       = var.environment == "production" ? 6 : 2
  min_capacity       = 1
  resource_id        = "service/${aws_ecs_cluster.main.name}/${each.value.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

resource "aws_appautoscaling_policy" "cpu_target" {
  for_each = aws_ecs_service.services

  name               = "${each.key}-cpu-target"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.ecs[each.key].resource_id
  scalable_dimension = aws_appautoscaling_target.ecs[each.key].scalable_dimension
  service_namespace  = aws_appautoscaling_target.ecs[each.key].service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }

    target_value       = 70
    scale_in_cooldown  = 60
    scale_out_cooldown = 60
  }
}
