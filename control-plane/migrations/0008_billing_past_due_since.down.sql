DROP INDEX IF EXISTS idx_tenant_settings_past_due_since;

ALTER TABLE tenant_settings
  DROP COLUMN IF EXISTS past_due_since;
