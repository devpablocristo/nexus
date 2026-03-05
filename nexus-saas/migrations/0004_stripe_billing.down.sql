DROP INDEX IF EXISTS idx_tenant_settings_stripe_customer;

ALTER TABLE tenant_settings
  DROP COLUMN IF EXISTS billing_status,
  DROP COLUMN IF EXISTS stripe_subscription_id,
  DROP COLUMN IF EXISTS stripe_customer_id;
