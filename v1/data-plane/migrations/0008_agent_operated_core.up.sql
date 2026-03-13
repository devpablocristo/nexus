CREATE TABLE IF NOT EXISTS operational_events (
  id bigserial PRIMARY KEY,
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  event_type text NOT NULL,
  payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_operational_events_org_id_id
  ON operational_events (org_id, id);

CREATE INDEX IF NOT EXISTS idx_operational_events_event_type_created_at
  ON operational_events (event_type, created_at DESC);

CREATE TABLE IF NOT EXISTS actions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  scope_type text NOT NULL CHECK (scope_type IN ('tenant', 'tool', 'agent', 'global')),
  scope_id text,
  action_type text NOT NULL CHECK (action_type IN ('throttle_tenant_rpm', 'throttle_tool_rpm', 'quarantine_tool', 'disable_tool')),
  params_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  ttl_seconds int NOT NULL DEFAULT 0 CHECK (ttl_seconds >= 0),
  status text NOT NULL CHECK (status IN ('active', 'expired', 'rolled_back')),
  evidence_refs_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by text,
  rolled_back_at timestamptz,
  rolled_back_by text
);

CREATE INDEX IF NOT EXISTS idx_actions_org_status_created_at
  ON actions (org_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_actions_org_scope
  ON actions (org_id, scope_type, scope_id);

CREATE TABLE IF NOT EXISTS incidents (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  severity text NOT NULL CHECK (severity IN ('LOW', 'MED', 'HIGH', 'CRIT')),
  status text NOT NULL CHECK (status IN ('open', 'closed')),
  title text NOT NULL,
  summary text NOT NULL,
  related_action_ids_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  evidence_refs_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_by text,
  opened_at timestamptz NOT NULL DEFAULT now(),
  closed_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_incidents_org_status_opened_at
  ON incidents (org_id, status, opened_at DESC);

CREATE TABLE IF NOT EXISTS policy_proposals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  status text NOT NULL CHECK (status IN ('draft', 'pending', 'approved', 'rejected', 'shadow')),
  diff_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  rationale text NOT NULL DEFAULT '',
  tests_suggested_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  rollback_plan text NOT NULL DEFAULT '',
  created_by text,
  created_at timestamptz NOT NULL DEFAULT now(),
  decided_by text,
  decided_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_policy_proposals_org_status_created_at
  ON policy_proposals (org_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS policy_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  proposal_id uuid REFERENCES policy_proposals(id) ON DELETE SET NULL,
  version_label text NOT NULL,
  spec_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  mode text NOT NULL CHECK (mode IN ('enforced', 'shadow')),
  created_by text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_policy_versions_org_created_at
  ON policy_versions (org_id, created_at DESC);

CREATE OR REPLACE FUNCTION emit_tool_call_operational_event()
RETURNS trigger AS $$
DECLARE
  payload jsonb;
BEGIN
  payload := jsonb_build_object(
    'request_id', NEW.request_id,
    'tool_name', NEW.tool_name,
    'decision', NEW.decision,
    'status', NEW.status,
    'latency_ms', NEW.latency_ms,
    'event_hash', NEW.event_hash,
    'dlp_summary', COALESCE(NEW.dlp_summary, '{}'::jsonb)
  );

  INSERT INTO operational_events (org_id, event_type, payload_json, created_at)
  VALUES (NEW.org_id, 'tool.call.completed', payload, NEW.created_at);

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_audit_to_operational_events ON audit_events;
CREATE TRIGGER trg_audit_to_operational_events
AFTER INSERT ON audit_events
FOR EACH ROW
EXECUTE PROCEDURE emit_tool_call_operational_event();
