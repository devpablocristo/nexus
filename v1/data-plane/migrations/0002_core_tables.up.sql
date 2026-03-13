CREATE TABLE IF NOT EXISTS orgs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS org_api_keys (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  api_key_hash text UNIQUE NOT NULL,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tools (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  name text NOT NULL,
  kind text NOT NULL CHECK (kind IN ('http')),
  description text,
  method text NOT NULL,
  url text NOT NULL,
  input_schema_json jsonb NOT NULL,
  output_schema_json jsonb,
  action_type text NOT NULL CHECK (action_type IN ('read','write')),
  risk_level int NOT NULL CHECK (risk_level BETWEEN 1 AND 5),
  enabled bool NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, name)
);

CREATE TABLE IF NOT EXISTS policies (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  tool_id uuid NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
  effect text NOT NULL CHECK (effect IN ('allow','deny')),
  priority int NOT NULL DEFAULT 100,
  conditions_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  limits_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  reason_template text NOT NULL DEFAULT '',
  enabled bool NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_policies_org_tool_priority ON policies (org_id, tool_id, priority);

CREATE TABLE IF NOT EXISTS audit_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  tool_id uuid NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
  tool_name text NOT NULL,
  request_id text UNIQUE NOT NULL,
  actor text,
  input_redacted jsonb NOT NULL,
  context_redacted jsonb NOT NULL,
  decision text NOT NULL CHECK (decision IN ('allow','deny')),
  policy_id uuid,
  reason text,
  status text NOT NULL CHECK (status IN ('success','error','blocked')),
  output_redacted jsonb,
  error_code text,
  error_message text,
  latency_ms int NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_org_created_at ON audit_events (org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_org_tool_created_at ON audit_events (org_id, tool_name, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_org_decision_status_created_at ON audit_events (org_id, decision, status, created_at DESC);

