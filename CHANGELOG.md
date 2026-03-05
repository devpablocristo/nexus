# Changelog

All notable changes to this project will be documented in this file.

## [0.9.0] - 2026-03-05

### Added
- User identity with Clerk (JWT, OIDC, webhooks)
- Billing with Stripe (checkout, portal, usage metering)
- Admin console (dashboard, settings, activity log)
- Email notifications (SES/SMTP, preferences, deduplication)
- Developer portal with OpenAPI, Postman, SDKs
- Production infrastructure (Terraform, ECS, RDS, CloudFront)
- Security hardening (CSP, HSTS, Dependabot, govulncheck)
- Monitoring (Prometheus, Grafana, alerting rules, SLO/SLI)
- Tenant lifecycle (suspend, reactivate, soft-delete)
- Load testing with k6
- Smoke test suite for production deploys
- Python, TypeScript, and Go SDKs

### Security
- Security headers on all services
- Non-root Docker containers
- Body size limits
- Dependency scanning in CI
