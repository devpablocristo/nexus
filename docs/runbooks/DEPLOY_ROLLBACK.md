# Deploy Rollback Procedure

## When to rollback

- Smoke test fails after deploy
- Error rate > 5% in first 15 minutes
- Critical bug reported in production

## ECS Rollback (preferred)

1. Identify the previous task definition revision:
   ```bash
   aws ecs describe-services --cluster nexus-prod --services nexus-core \
     --query 'services[0].taskDefinition'
   ```

2. Update service to previous revision:
   ```bash
   aws ecs update-service --cluster nexus-prod \
     --service nexus-core \
     --task-definition nexus-core:<PREVIOUS_REVISION> \
     --force-new-deployment
   ```

3. Wait for stable:
   ```bash
   aws ecs wait services-stable --cluster nexus-prod --services nexus-core
   ```

4. Run smoke test:
   ```bash
   ./scripts/smoke/smoke_prod.sh https://api.nexus.io https://saas.nexus.io https://app.nexus.io
   ```

## Database rollback

If the deploy included a migration:

1. Check the current migration version:
   ```sql
   SELECT version, dirty FROM schema_migrations;
   ```

2. Run the down migration:
   ```bash
   migrate -path migrations/ -database "$DATABASE_URL" down 1
   ```

3. **WARNING**: Down migrations may cause data loss. Always review the `.down.sql` first.

## CloudFront rollback (frontend)

1. Re-deploy previous S3 assets from ECR image:
   ```bash
   aws s3 sync s3://nexus-assets-backup/ s3://nexus-assets-prod/
   ```

2. Invalidate CDN cache:
   ```bash
   aws cloudfront create-invalidation --distribution-id $CF_DIST_ID --paths "/*"
   ```

## Post-rollback

- [ ] Verify all smoke tests pass
- [ ] Notify team in #incidents channel
- [ ] Create post-mortem ticket
- [ ] Disable failed deploy branch protection if needed
