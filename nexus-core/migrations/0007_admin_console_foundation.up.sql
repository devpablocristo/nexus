CREATE TABLE IF NOT EXISTS tenant_settings (
  org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
  plan_code text NOT NULL DEFAULT 'starter' CHECK (plan_code IN ('starter','growth','enterprise')),
  hard_limits_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  updated_by text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS admin_activity_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  actor text,
  action text NOT NULL,
  resource_type text NOT NULL,
  resource_id text,
  payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_admin_activity_org_created_at
  ON admin_activity_events (org_id, created_at DESC);

DROP TRIGGER IF EXISTS trg_tenant_settings_set_updated_at ON tenant_settings;
CREATE TRIGGER trg_tenant_settings_set_updated_at
BEFORE UPDATE ON tenant_settings
FOR EACH ROW
EXECUTE PROCEDURE set_updated_at();
