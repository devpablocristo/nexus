ALTER TABLE tools
  ADD COLUMN IF NOT EXISTS classification text NOT NULL DEFAULT 'internal' CHECK (classification IN ('internal','external'));

CREATE TABLE IF NOT EXISTS org_api_key_scopes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_key_id uuid NOT NULL REFERENCES org_api_keys(id) ON DELETE CASCADE,
  scope text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (api_key_id, scope)
);

CREATE TABLE IF NOT EXISTS tool_secrets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  tool_id uuid NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
  secret_type text NOT NULL CHECK (secret_type IN ('header','bearer')),
  key_name text NOT NULL DEFAULT '',
  ciphertext bytea NOT NULL,
  nonce bytea NOT NULL,
  enabled bool NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, tool_id, key_name)
);

CREATE TABLE IF NOT EXISTS tool_egress_rules (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  tool_id uuid NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
  host text NOT NULL,
  enabled bool NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, tool_id, host)
);

ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS dlp_summary jsonb NOT NULL DEFAULT '{}'::jsonb;

DROP TRIGGER IF EXISTS trg_tool_secrets_set_updated_at ON tool_secrets;
CREATE TRIGGER trg_tool_secrets_set_updated_at
BEFORE UPDATE ON tool_secrets
FOR EACH ROW
EXECUTE PROCEDURE set_updated_at();
