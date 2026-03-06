# Production Launch Checklist

## Pre-launch
- [ ] All Terraform modules applied (staging first, then prod)
- [ ] DNS configured (Route53 -> ALB, CloudFront)
- [ ] TLS certificates provisioned (ACM)
- [ ] Clerk production instance configured
- [ ] Stripe production keys + webhooks configured
- [ ] SES production access (out of sandbox)
- [ ] Secrets in AWS Secrets Manager
- [ ] Database migrations applied
- [ ] Seed data loaded (if needed)

## Deploy
- [ ] CI green on main
- [ ] Docker images pushed to ECR
- [ ] ECS services updated
- [ ] CloudFront invalidation complete

## Post-deploy validation
- [ ] Run smoke_prod.sh against production URLs
- [ ] Verify health endpoints for all services
- [ ] Verify Prometheus targets are UP
- [ ] Verify Grafana dashboards show data
- [ ] Test user sign-up flow (Clerk)
- [ ] Test billing flow (Stripe test mode -> live)
- [ ] Test tool registration + execution
- [ ] Test email delivery (SES)

## Monitoring
- [ ] CloudWatch alarms configured and SNS subscribed
- [ ] Grafana alerts configured
- [ ] On-call rotation defined
- [ ] Incident response runbook reviewed
- [ ] `docs/testing/RELEASE_GATES.md` reviewed for this release scope
- [ ] `docs/runbooks/INCIDENT_RESPONSE.md` reviewed by on-call owner

## Security
- [ ] Rotate all development secrets
- [ ] CORS origins set to production domains only
- [ ] CSP connect-src updated with production URLs
- [ ] Verify non-root containers
- [ ] Review Dependabot PRs
- [ ] Rollback path confirmed in `docs/runbooks/DEPLOY_ROLLBACK.md`
