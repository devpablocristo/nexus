ALTER TABLE tenant_settings
  ADD COLUMN IF NOT EXISTS stripe_customer_id text UNIQUE,
  ADD COLUMN IF NOT EXISTS stripe_subscription_id text UNIQUE,
  ADD COLUMN IF NOT EXISTS billing_status text NOT NULL DEFAULT 'trialing'
    CHECK (billing_status IN ('trialing','active','past_due','canceled','unpaid'));

CREATE INDEX IF NOT EXISTS idx_tenant_settings_stripe_customer
  ON tenant_settings(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;
