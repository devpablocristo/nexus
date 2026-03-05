# Secret Rotation Runbook

## Scope

This runbook defines how to rotate high-impact secrets used by Nexus:
- `NEXUS_MASTER_KEY`
- `NEXUS_CORE_INTERNAL_KEY`
- `NEXUS_SAAS_INTERNAL_KEY`
- `CLERK_SECRET_KEY`
- `CLERK_WEBHOOK_SECRET`
- `STRIPE_SECRET_KEY`
- `STRIPE_WEBHOOK_SECRET`
- Database credentials (`NEXUS_DATABASE_URL`, `NEXUS_SAAS_DATABASE_URL`)

Target runtime: AWS Secrets Manager + ECS services (`nexus-core`, `nexus-saas`, `nexus-ai-operators`, `nexus-tower`).

## Rotation Principles

1. Rotate with overlap when possible (dual-read or coordinated deploy).
2. Prefer staged rollout: `staging` first, then `production`.
3. Keep audit evidence: ticket ID, who rotated, timestamp, old version deactivation time.
4. Validate health and critical flows immediately after deploy.

## 1) `NEXUS_MASTER_KEY` (AES-GCM for tool secrets)

Risk: decrypt/encrypt failures if switched without re-encryption strategy.

Procedure:
1. Generate a new 32-byte key:
   ```bash
   openssl rand -base64 32
   ```
2. Put key in Secrets Manager as a **new version** (do not remove old yet).
3. Run a controlled re-encryption job:
   - Read each encrypted secret with old key.
   - Re-encrypt with new key.
   - Store updated ciphertext.
4. Deploy `nexus-core` and any service decrypting tool secrets with the new key.
5. Validate:
   - create/update secret
   - execute tool requiring injected secret
6. Disable previous key version after verification window.

## 2) Internal keys (`NEXUS_CORE_INTERNAL_KEY` / `NEXUS_SAAS_INTERNAL_KEY`)

Risk: cross-service auth breaks if only one side rotates.

Procedure:
1. Generate new key value:
   ```bash
   openssl rand -hex 32
   ```
2. Update Secrets Manager for both services.
3. Deploy both sides in the same change window:
   - `nexus-core`
   - `nexus-saas`
   - any internal client using these headers
4. Validate internal endpoints:
   - `/internal/entitlements/:org_id`
   - `/internal/runtime-overrides/:org_id/:tool_name`
5. Revoke old key.

## 3) Clerk (`CLERK_SECRET_KEY`, `CLERK_WEBHOOK_SECRET`)

Procedure:
1. Rotate in Clerk Dashboard/API.
2. Update Secrets Manager with new values.
3. Deploy `nexus-saas` (webhook verification + backend calls).
4. Validate:
   - user sign-in
   - Clerk webhook delivery (`/v1/webhooks/clerk`)
5. Remove old key in Clerk after successful validation.

## 4) Stripe (`STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`)

Procedure:
1. Rotate in Stripe Dashboard.
2. Update Secrets Manager with new values.
3. Deploy `nexus-saas`.
4. Validate:
   - `POST /v1/billing/checkout`
   - Stripe webhook receipt (`/v1/webhooks/stripe`)
5. Deactivate old key/webhook secret in Stripe.

## 5) Database credentials (RDS)

Procedure:
1. Rotate credentials in RDS (or through Secrets Manager integration).
2. Update connection strings in Secrets Manager:
   - `NEXUS_DATABASE_URL`
   - `NEXUS_SAAS_DATABASE_URL`
3. Redeploy services consuming those URLs.
4. Validate:
   - `GET /readyz` (`nexus-core`)
   - `GET /health` (`nexus-saas`)
5. Revoke old credentials.

## Emergency Rotation (Compromise)

Trigger examples:
- secret leaked in logs/chat/repo
- unauthorized API activity
- provider compromise notice

Immediate actions:
1. Declare incident and freeze risky deploys.
2. Rotate compromised secret first (no batching).
3. Force restart affected services.
4. Revoke previous credentials/keys immediately.
5. Review audit logs and access logs for misuse window.
6. Create post-incident report with timeline + remediation.

## Validation Checklist (Post-Rotation)

1. All service health endpoints are green.
2. Auth flows work (JWT/API key/webhooks).
3. Billing and notifications still process events.
4. No sustained 401/403/5xx regression after rotation.

## Metrics Exposure Note

- `/metrics` endpoints are for internal scraping only.
- `nexus-ai-operators` metrics must be protected with API key.
- `nexus-core` and `nexus-saas` metrics must not be publicly exposed; allow only internal VPC/monitoring access paths.
