DROP TRIGGER IF EXISTS trg_tenant_settings_set_updated_at ON tenant_settings;
DROP INDEX IF EXISTS idx_admin_activity_org_created_at;
DROP TABLE IF EXISTS admin_activity_events;
DROP TABLE IF EXISTS tenant_settings;
