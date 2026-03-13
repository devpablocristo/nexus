ALTER TABLE tenant_settings
  ADD COLUMN IF NOT EXISTS past_due_since timestamptz;

UPDATE tenant_settings
SET past_due_since = updated_at
WHERE billing_status = 'past_due'
  AND past_due_since IS NULL;

CREATE INDEX IF NOT EXISTS idx_tenant_settings_past_due_since
  ON tenant_settings(past_due_since)
  WHERE billing_status = 'past_due';
