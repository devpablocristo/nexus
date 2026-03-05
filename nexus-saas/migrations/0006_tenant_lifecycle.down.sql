ALTER TABLE tenant_settings
  DROP COLUMN IF EXISTS deleted_at,
  DROP COLUMN IF EXISTS status;
