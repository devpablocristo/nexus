# Nexus v2 AWS Infrastructure

This directory defines the AWS baseline for `v2`.

Current scope:
- `data-plane`
- `control-plane`
- `control-workers`
- shared PostgreSQL on RDS
- shared ECS cluster on Fargate
- public ALB for `data-plane` and `control-plane`
- private DNS service discovery for service-to-service traffic
- ECR repositories
- Secrets Manager placeholders for runtime credentials
- generated API keys in Secrets Manager by default

Deliberately out of scope for this first cut:
- `ai-runtime`
- CDN
- Redis/cache
- Route53/DNS records
- WAF
- multi-region

## Layout

- `main.tf`: root wiring
- `variables.tf`: root inputs
- `outputs.tf`: root outputs
- `backend.tf`: remote state backend declaration
- `environments/`: example per-environment values
- `modules/`: local reusable AWS modules

## Usage

1. Copy an environment file and fill the placeholders.
2. Create a backend config file from `backend.hcl.example`.
3. Initialize Terraform.
4. Review the plan.
5. Apply in the target AWS account.

Example:

```bash
cd v2/infra
terraform init -backend-config=backend.hcl
terraform plan -var-file=environments/staging.tfvars
terraform apply -var-file=environments/staging.tfvars
```

## API keys and secrets

- The source of truth for staging and production API keys is AWS Secrets Manager.
- By default, Terraform generates:
  - admin API key
  - data-plane service API key
  - control-workers service API key
  - Prometheus scrape API key
- Each key is stored both:
  - as an individual secret for operator retrieval and rotation
  - as aggregated runtime secrets consumed by ECS tasks
- If you need to inject existing keys, set the optional root variables:
  - `admin_api_key`
  - `data_plane_service_api_key`
  - `control_workers_service_api_key`
  - `prometheus_api_key`
- ECS tasks read runtime credentials from Secrets Manager at boot.
- Human access to admin endpoints should retrieve the admin secret from Secrets Manager with IAM, not from committed files.

Useful commands after apply:

```bash
terraform output api_key_secret_arns
terraform output runtime_secret_arns
```

## Notes

- The root module intentionally keeps runtime secrets in Secrets Manager and injects them into ECS task definitions.
- Internal service URLs use Cloud Map private DNS:
  - `http://control-plane.<namespace>:8081`
  - `http://control-workers.<namespace>:8082`
- The first infra cut uses a single PostgreSQL instance and a single logical database for the three services plus audit. That keeps the MVP small and matches the current table names, which do not collide.
- `docker compose` remains dev-only. Pre-prod and prod should source secrets from AWS, not from `.env`.
