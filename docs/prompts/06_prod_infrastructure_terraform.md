# Prompt 06 — Production Infrastructure (Terraform + AWS) & DB Backup/DR

## Contexto del proyecto

Nexus es una plataforma SaaS con estos servicios:

| Servicio | Stack | Puerto | Health | DB | Redis | Externos |
|----------|-------|--------|--------|----|-------|----------|
| nexus-core | Go/Gin + Alpine | 8080 | GET /readyz | PostgreSQL `nexus` | Sí (rate-limit) | — |
| nexus-saas | Go/Gin + Alpine | 8082 | GET /health | PostgreSQL `nexus_saas` | No | Clerk, Stripe, SES |
| nexus-tower | Nginx + Vite build | 4173 | GET / | — | — | Clerk (frontend) |
| nexus-control-operators | Go + Alpine | 8090 | GET /healthz | — (file-based state) | — | — |
| nexus-ai-operators | Python/FastAPI | 8000 | GET /readyz | — | — | Anthropic |
| mock-tools | Go + Alpine | 8081 | GET /healthz | — | — | — (dev only) |

**Stack decidido**: AWS completo + Clerk + Stripe.

**Repositorio**: monorepo con Go workspace, cada servicio tiene su `Dockerfile`.

---

## Lo que YA existe

- Docker Compose para desarrollo local (`docker-compose.yml`, `docker-compose.dev.yml`)
- Dockerfiles de producción para todos los servicios (multi-stage, Alpine-based)
- GitHub Actions CI: tests, build, e2e (`ci.yml`)
- Health checks en todos los servicios
- Prometheus + Grafana para monitoreo local
- 2 bases PostgreSQL separadas (nexus, nexus_saas)
- Redis para rate-limiting en nexus-core
- **NO existe**: Terraform, CDK, deployment scripts, CI/CD de producción, configuración de AWS

---

## Qué implementar

### Fase 1 — Estructura Terraform

Crear `infra/` en la raíz del proyecto:

```
infra/
├── main.tf                    # Root module, backend config, providers
├── variables.tf               # Variables globales (region, environment, domain, etc.)
├── outputs.tf                 # Outputs principales (URLs, endpoints)
├── terraform.tfvars.example   # Ejemplo de variables (sin secretos)
├── environments/
│   ├── staging.tfvars         # Override vars para staging
│   └── production.tfvars      # Override vars para producción
├── modules/
│   ├── networking/
│   │   ├── main.tf            # VPC, subnets, NAT, IGW, security groups
│   │   ├── variables.tf
│   │   └── outputs.tf
│   ├── database/
│   │   ├── main.tf            # RDS PostgreSQL (2 instancias), backup, PITR
│   │   ├── variables.tf
│   │   └── outputs.tf
│   ├── cache/
│   │   ├── main.tf            # ElastiCache Redis
│   │   ├── variables.tf
│   │   └── outputs.tf
│   ├── ecs/
│   │   ├── main.tf            # ECS Cluster, task definitions, services
│   │   ├── variables.tf
│   │   └── outputs.tf
│   ├── loadbalancer/
│   │   ├── main.tf            # ALB, target groups, listeners, HTTPS
│   │   ├── variables.tf
│   │   └── outputs.tf
│   ├── cdn/
│   │   ├── main.tf            # S3 + CloudFront para nexus-tower
│   │   ├── variables.tf
│   │   └── outputs.tf
│   ├── dns/
│   │   ├── main.tf            # Route53 hosted zone, records
│   │   ├── variables.tf
│   │   └── outputs.tf
│   ├── secrets/
│   │   ├── main.tf            # Secrets Manager para todas las secrets
│   │   ├── variables.tf
│   │   └── outputs.tf
│   ├── monitoring/
│   │   ├── main.tf            # CloudWatch log groups, alarms, dashboards
│   │   ├── variables.tf
│   │   └── outputs.tf
│   └── ecr/
│       ├── main.tf            # ECR repositories para cada servicio
│       ├── variables.tf
│       └── outputs.tf
```

---

### Fase 2 — Módulos Terraform detallados

#### 2.1 Networking (`modules/networking/`)

```hcl
# VPC con 2 AZs
resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr  # default: "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true
}

# 2 subnets públicas (ALB, NAT)
# 2 subnets privadas (ECS tasks, RDS, ElastiCache)
# Internet Gateway
# NAT Gateway (1 por AZ o 1 shared para ahorrar)
# Route tables

# Security groups:
# - sg_alb: 80, 443 desde 0.0.0.0/0
# - sg_ecs: 8080, 8082, 8090, 8000 desde sg_alb
# - sg_rds: 5432 desde sg_ecs
# - sg_redis: 6379 desde sg_ecs
```

Variables:

```hcl
variable "vpc_cidr" { default = "10.0.0.0/16" }
variable "azs" { default = ["us-east-1a", "us-east-1b"] }
variable "environment" { type = string }
```

#### 2.2 Database (`modules/database/`)

**2 instancias RDS PostgreSQL** (una para nexus-core, otra para nexus-saas):

```hcl
resource "aws_db_instance" "nexus_core" {
  identifier     = "${var.environment}-nexus-core"
  engine         = "postgres"
  engine_version = "16.4"
  instance_class = var.core_db_instance_class  # default: "db.t4g.micro"

  db_name  = "nexus"
  username = "nexus_core"
  password = var.core_db_password  # from Secrets Manager

  allocated_storage     = 20
  max_allocated_storage = 100
  storage_encrypted     = true

  # BACKUP & DR
  backup_retention_period = 7           # 7 días de backups automáticos
  backup_window           = "03:00-04:00"  # UTC
  maintenance_window      = "Mon:04:00-Mon:05:00"
  copy_tags_to_snapshot   = true
  deletion_protection     = true
  skip_final_snapshot     = false
  final_snapshot_identifier = "${var.environment}-nexus-core-final"

  # HIGH AVAILABILITY
  multi_az = var.environment == "production"

  # NETWORKING
  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [var.sg_rds_id]
  publicly_accessible    = false

  # PITR (Point-in-Time Recovery)
  # Habilitado automáticamente con backup_retention_period > 0

  performance_insights_enabled = true
}

resource "aws_db_instance" "nexus_saas" {
  identifier     = "${var.environment}-nexus-saas"
  engine         = "postgres"
  engine_version = "16.4"
  instance_class = var.saas_db_instance_class  # default: "db.t4g.micro"

  db_name  = "nexus_saas"
  username = "nexus_saas"
  password = var.saas_db_password

  # misma config de backup, encryption, multi-az, etc.
  backup_retention_period   = 7
  backup_window             = "03:00-04:00"
  storage_encrypted         = true
  multi_az                  = var.environment == "production"
  deletion_protection       = true
  skip_final_snapshot       = false
  final_snapshot_identifier = "${var.environment}-nexus-saas-final"
  performance_insights_enabled = true

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [var.sg_rds_id]
  publicly_accessible    = false
}
```

#### 2.3 Cache (`modules/cache/`)

```hcl
resource "aws_elasticache_replication_group" "redis" {
  replication_group_id = "${var.environment}-nexus-redis"
  description          = "Nexus Redis for rate-limiting"
  engine               = "redis"
  engine_version       = "7.0"
  node_type            = var.redis_node_type  # default: "cache.t4g.micro"
  num_cache_clusters   = var.environment == "production" ? 2 : 1
  
  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  
  subnet_group_name  = aws_elasticache_subnet_group.main.name
  security_group_ids = [var.sg_redis_id]
}
```

#### 2.4 ECR (`modules/ecr/`)

Un repositorio por servicio:

```hcl
locals {
  services = ["nexus-core", "nexus-saas", "nexus-control-operators", "nexus-ai-operators"]
}

resource "aws_ecr_repository" "services" {
  for_each = toset(local.services)
  name     = "${var.environment}/${each.key}"

  image_scanning_configuration {
    scan_on_push = true
  }

  image_tag_mutability = "IMMUTABLE"
}

resource "aws_ecr_lifecycle_policy" "cleanup" {
  for_each   = aws_ecr_repository.services
  repository = each.value.name
  policy     = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Keep last 20 images"
      selection    = { tagStatus = "any", countType = "imageCountMoreThan", countNumber = 20 }
      action       = { type = "expire" }
    }]
  })
}
```

#### 2.5 ECS (`modules/ecs/`)

Cluster ECS Fargate con 4 servicios:

```hcl
resource "aws_ecs_cluster" "main" {
  name = "${var.environment}-nexus"
  
  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}
```

**Task definitions** (una por servicio):

| Servicio | CPU | Memory | Port | Health check |
|----------|-----|--------|------|-------------|
| nexus-core | 512 | 1024 | 8080 | GET /readyz |
| nexus-saas | 512 | 1024 | 8082 | GET /health |
| nexus-control-operators | 256 | 512 | 8090 | GET /healthz |
| nexus-ai-operators | 512 | 1024 | 8000 | GET /readyz |

Cada task definition:
- Usa imagen de ECR
- Inyecta secrets desde Secrets Manager (vía `secrets` en container definition)
- Envía logs a CloudWatch (`awslogs` driver)
- Health check configurado

**ECS services** con:
- `desired_count = 2` para producción, `1` para staging
- Auto-scaling basado en CPU (target 70%)
- Deployment circuit breaker habilitado
- Rolling update (min 50%, max 200%)

#### 2.6 Load Balancer (`modules/loadbalancer/`)

ALB con:
- Listener HTTPS (443) con certificado ACM
- Listener HTTP (80) con redirect a HTTPS
- Target groups por servicio:

| Target group | Path pattern | Puerto | Service |
|-------------|-------------|--------|---------|
| tg-core | /v1/run*, /v1/tools*, /v1/policies*, /v1/audit*, /v1/secrets*, /v1/approvals*, /mcp, /a2a/* | 8080 | nexus-core |
| tg-saas | /v1/admin*, /v1/billing*, /v1/incidents*, /v1/notifications*, /v1/users*, /v1/events*, /v1/actions*, /v1/alert-rules*, /v1/sessions*, /v1/policy-proposals*, /v1/assistant*, /v1/orgs*, /v1/auth/*, /v1/webhooks/*, /internal/* | 8082 | nexus-saas |
| tg-core (fallback) | /openapi.yaml, /docs, /healthz, /readyz | 8080 | nexus-core |

**Nota**: nexus-control-operators y nexus-ai-operators son internos, no expuestos en el ALB. Se comunican via service discovery de ECS (Cloud Map) o por IP privada.

#### 2.7 CDN (`modules/cdn/`)

S3 + CloudFront para nexus-tower:

```hcl
resource "aws_s3_bucket" "tower" {
  bucket = "${var.environment}-nexus-tower"
}

resource "aws_cloudfront_distribution" "tower" {
  origin {
    domain_name = aws_s3_bucket.tower.bucket_regional_domain_name
    origin_id   = "tower-s3"
    s3_origin_config {
      origin_access_identity = aws_cloudfront_origin_access_identity.tower.cloudfront_access_identity_path
    }
  }

  default_cache_behavior {
    allowed_methods  = ["GET", "HEAD"]
    cached_methods   = ["GET", "HEAD"]
    target_origin_id = "tower-s3"
    
    forwarded_values {
      query_string = false
      cookies { forward = "none" }
    }

    viewer_protocol_policy = "redirect-to-https"
  }

  # SPA: custom error response para 404 → /index.html
  custom_error_response {
    error_code         = 404
    response_code      = 200
    response_page_path = "/index.html"
  }

  default_root_object = "index.html"
  
  viewer_certificate {
    acm_certificate_arn = var.certificate_arn
    ssl_support_method  = "sni-only"
  }
}
```

#### 2.8 DNS (`modules/dns/`)

Route53:
- `api.nexus.example.com` → ALB
- `app.nexus.example.com` → CloudFront
- ACM certificate con SANs para ambos

#### 2.9 Secrets (`modules/secrets/`)

Secrets Manager entries:

| Secret | Servicios |
|--------|-----------|
| `nexus-core-db-password` | nexus-core |
| `nexus-saas-db-password` | nexus-saas |
| `nexus-master-key` | nexus-core |
| `nexus-saas-internal-key` | nexus-core, nexus-saas |
| `nexus-operator-api-key` | nexus-core, operators |
| `clerk-secret-key` | nexus-saas |
| `clerk-webhook-secret` | nexus-saas |
| `stripe-secret-key` | nexus-saas |
| `stripe-webhook-secret` | nexus-saas |
| `anthropic-api-key` | nexus-ai-operators |

Cada secret se inyecta en ECS tasks vía `secrets` (no environment variables).

#### 2.10 Monitoring (`modules/monitoring/`)

- CloudWatch Log Groups para cada servicio (retención 30 días)
- CloudWatch Alarms:
  - ECS CPU > 80%
  - ECS Memory > 80%
  - RDS CPU > 80%
  - RDS Free Storage < 5GB
  - ALB 5xx > 10 en 5 min
  - ALB target unhealthy
- SNS Topic para alertas (email)

---

### Fase 3 — Variables globales

`infra/variables.tf`:

```hcl
variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "environment" {
  type        = string
  description = "staging or production"
}

variable "domain" {
  type        = string
  description = "Base domain (e.g. nexus.example.com)"
}

variable "vpc_cidr" {
  type    = string
  default = "10.0.0.0/16"
}

# DB
variable "core_db_instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "saas_db_instance_class" {
  type    = string
  default = "db.t4g.micro"
}

# Cache
variable "redis_node_type" {
  type    = string
  default = "cache.t4g.micro"
}

# ECS
variable "core_desired_count" {
  type    = number
  default = 1
}

variable "saas_desired_count" {
  type    = number
  default = 1
}

# Secrets (passed from CI/CD, never in tfvars)
variable "core_db_password" {
  type      = string
  sensitive = true
}

variable "saas_db_password" {
  type      = string
  sensitive = true
}
```

`infra/terraform.tfvars.example`:

```hcl
aws_region             = "us-east-1"
environment            = "staging"
domain                 = "nexus.example.com"
vpc_cidr               = "10.0.0.0/16"
core_db_instance_class = "db.t4g.micro"
saas_db_instance_class = "db.t4g.micro"
redis_node_type        = "cache.t4g.micro"
core_desired_count     = 1
saas_desired_count     = 1
```

`infra/environments/staging.tfvars`:

```hcl
environment        = "staging"
core_desired_count = 1
saas_desired_count = 1
```

`infra/environments/production.tfvars`:

```hcl
environment            = "production"
core_db_instance_class = "db.t4g.small"
saas_db_instance_class = "db.t4g.small"
redis_node_type        = "cache.t4g.small"
core_desired_count     = 2
saas_desired_count     = 2
```

---

### Fase 4 — CI/CD de producción

Crear `.github/workflows/deploy.yml`:

```yaml
name: deploy

on:
  push:
    branches: [main]
    tags: ["v*"]
  workflow_dispatch:
    inputs:
      environment:
        description: "Target environment"
        required: true
        type: choice
        options: [staging, production]

concurrency:
  group: deploy-${{ github.event.inputs.environment || 'staging' }}
  cancel-in-progress: false

env:
  AWS_REGION: us-east-1

jobs:
  build-and-push:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    strategy:
      matrix:
        service: [nexus-core, nexus-saas, nexus-control-operators, nexus-ai-operators]
    steps:
      - uses: actions/checkout@v4

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-arn: ${{ secrets.AWS_DEPLOY_ROLE_ARN }}
          aws-region: ${{ env.AWS_REGION }}

      - name: Login to ECR
        uses: aws-actions/amazon-ecr-login@v2

      - name: Build and push
        env:
          ECR_REGISTRY: ${{ steps.login-ecr.outputs.registry }}
          IMAGE_TAG: ${{ github.sha }}
        run: |
          docker build \
            -f ${{ matrix.service }}/Dockerfile \
            -t $ECR_REGISTRY/${{ matrix.service }}:$IMAGE_TAG \
            -t $ECR_REGISTRY/${{ matrix.service }}:latest \
            .
          docker push $ECR_REGISTRY/${{ matrix.service }}:$IMAGE_TAG
          docker push $ECR_REGISTRY/${{ matrix.service }}:latest

  build-tower:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "20"
      - name: Install and build
        working-directory: nexus-tower
        env:
          VITE_NEXUS_CORE_URL: ${{ vars.VITE_NEXUS_CORE_URL }}
          VITE_NEXUS_SAAS_URL: ${{ vars.VITE_NEXUS_SAAS_URL }}
          VITE_CLERK_PUBLISHABLE_KEY: ${{ vars.VITE_CLERK_PUBLISHABLE_KEY }}
        run: |
          npm ci
          npm run build

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-arn: ${{ secrets.AWS_DEPLOY_ROLE_ARN }}
          aws-region: ${{ env.AWS_REGION }}

      - name: Sync to S3
        run: |
          aws s3 sync nexus-tower/dist/ \
            s3://${{ vars.TOWER_S3_BUCKET }}/ \
            --delete

      - name: Invalidate CloudFront
        run: |
          aws cloudfront create-invalidation \
            --distribution-id ${{ vars.CLOUDFRONT_DISTRIBUTION_ID }} \
            --paths "/*"

  deploy-ecs:
    runs-on: ubuntu-latest
    needs: [build-and-push]
    permissions:
      id-token: write
      contents: read
    strategy:
      matrix:
        service: [nexus-core, nexus-saas, nexus-control-operators, nexus-ai-operators]
    steps:
      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-arn: ${{ secrets.AWS_DEPLOY_ROLE_ARN }}
          aws-region: ${{ env.AWS_REGION }}

      - name: Update ECS service
        run: |
          aws ecs update-service \
            --cluster ${{ vars.ECS_CLUSTER }} \
            --service ${{ matrix.service }} \
            --force-new-deployment
```

---

### Fase 5 — Backend remoto de Terraform

En `infra/main.tf`:

```hcl
terraform {
  required_version = ">= 1.5"

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
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "nexus"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}
```

---

### Fase 6 — DB Backup & DR Documentation

Crear `docs/runbooks/DB_BACKUP_DR.md`:

```markdown
# Database Backup & Disaster Recovery

## Automated Backups (RDS)

- Retention: 7 days
- Window: 03:00-04:00 UTC daily
- Point-in-Time Recovery (PITR): enabled (5-minute granularity)
- Encryption: AES-256 at rest
- Multi-AZ: production only

## Recovery Procedures

### Restore to Point in Time

1. Go to RDS Console → Select instance → Actions → Restore to point in time
2. Choose target time (up to 5 minutes before current)
3. Configure new instance settings
4. Launch and update ECS task definitions with new endpoint

CLI:
aws rds restore-db-instance-to-point-in-time \
  --source-db-instance-identifier production-nexus-core \
  --target-db-instance-identifier production-nexus-core-restored \
  --restore-time "2025-01-15T10:30:00Z"

### Restore from Snapshot

1. RDS Console → Snapshots → Select snapshot → Restore snapshot
2. Configure instance settings
3. Update DNS/ECS to point to new instance

### Manual Snapshot (before risky operations)

aws rds create-db-snapshot \
  --db-instance-identifier production-nexus-core \
  --db-snapshot-identifier manual-pre-migration-$(date +%Y%m%d)

## Monitoring

- CloudWatch Alarm: FreeStorageSpace < 5GB
- CloudWatch Alarm: CPUUtilization > 80%
- Backup verification: check RDS automated backups weekly

## RTO / RPO

| Metric | Target |
|--------|--------|
| RPO (data loss) | ≤ 5 minutes (PITR) |
| RTO (recovery time) | ≤ 30 minutes (snapshot restore) |

## Runbook: Complete DB Failure

1. Identify failure via CloudWatch alarm or health check
2. If Multi-AZ: automatic failover (2-3 minutes)
3. If single-AZ: restore from latest snapshot or PITR
4. Run pending migrations: `make migrate-up`
5. Verify via health checks
6. Notify team via incident channel
```

---

## Reglas de implementación

1. **Terraform**: usar HCL puro, no wrappers ni CDK. Terraform >= 1.5, AWS provider ~> 5.0.
2. **Módulos**: cada módulo es independiente con sus `variables.tf` y `outputs.tf`.
3. **Secrets**: NUNCA en `.tfvars` ni en código. Usar `sensitive = true` en variables y Secrets Manager para runtime.
4. **State**: S3 backend con DynamoDB locking. El bucket y tabla de DynamoDB se crean manualmente (bootstrap).
5. **Naming**: `${var.environment}-nexus-*` para todos los recursos.
6. **Tags**: todos los recursos tagueados con Project, Environment, ManagedBy.
7. **Security groups**: least-privilege. Solo puertos necesarios entre componentes.
8. **Encryption**: at rest y in transit para todo (RDS, Redis, S3, ECS).
9. **No incluir**: mock-tools, ollama, prometheus, grafana (son dev-only o se reemplazan con CloudWatch).
10. **ECS Service Discovery**: usar Cloud Map para comunicación interna entre servicios (nexus-core ↔ nexus-saas, core ↔ operators).

---

## Criterios de éxito

- [ ] `infra/` directory con todos los módulos Terraform
- [ ] `terraform validate` pasa sin errores
- [ ] `terraform plan` genera un plan válido (con variables dummy)
- [ ] Módulo networking: VPC, 4 subnets, IGW, NAT, 4 security groups
- [ ] Módulo database: 2 RDS PostgreSQL con backup 7d, PITR, multi-AZ (prod), encryption
- [ ] Módulo cache: ElastiCache Redis con encryption
- [ ] Módulo ecr: 4 repositorios con lifecycle policy
- [ ] Módulo ecs: cluster + 4 task definitions + 4 services + auto-scaling
- [ ] Módulo loadbalancer: ALB con HTTPS, path-based routing a core y saas
- [ ] Módulo cdn: S3 + CloudFront para nexus-tower con SPA support
- [ ] Módulo dns: Route53 records
- [ ] Módulo secrets: Secrets Manager entries
- [ ] Módulo monitoring: CloudWatch log groups + alarms + SNS
- [ ] `.github/workflows/deploy.yml` con build→push→deploy pipeline
- [ ] `terraform.tfvars.example` documentado
- [ ] `environments/staging.tfvars` y `environments/production.tfvars`
- [ ] `docs/runbooks/DB_BACKUP_DR.md` con procedimientos de recovery
- [ ] Outputs: ALB DNS, CloudFront domain, RDS endpoints, ECR URLs
- [ ] Sin secretos hardcodeados en ningún archivo
- [ ] `infra/.gitignore` con `*.tfstate`, `*.tfvars` (excepto example), `.terraform/`

---

## Orden de ejecución recomendado

1. Crear `infra/.gitignore`
2. Crear `infra/variables.tf` y `infra/main.tf` (backend, provider)
3. Módulo `networking/` (VPC, subnets, security groups)
4. Módulo `database/` (RDS × 2)
5. Módulo `cache/` (ElastiCache)
6. Módulo `ecr/` (repositorios)
7. Módulo `secrets/` (Secrets Manager)
8. Módulo `ecs/` (cluster, tasks, services)
9. Módulo `loadbalancer/` (ALB, routing)
10. Módulo `cdn/` (S3 + CloudFront)
11. Módulo `dns/` (Route53)
12. Módulo `monitoring/` (CloudWatch)
13. `infra/outputs.tf`
14. `.github/workflows/deploy.yml`
15. `infra/terraform.tfvars.example`
16. `infra/environments/`
17. `docs/runbooks/DB_BACKUP_DR.md`
18. Verificar con `terraform fmt` y `terraform validate`
