DROP TRIGGER IF EXISTS trg_tool_secrets_set_updated_at ON tool_secrets;

ALTER TABLE audit_events DROP COLUMN IF EXISTS dlp_summary;
DROP TABLE IF EXISTS tool_egress_rules;
DROP TABLE IF EXISTS tool_secrets;
DROP TABLE IF EXISTS org_api_key_scopes;
ALTER TABLE tools DROP COLUMN IF EXISTS classification;
