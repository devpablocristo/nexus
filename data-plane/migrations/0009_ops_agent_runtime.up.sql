CREATE TABLE IF NOT EXISTS ops_event_store (
  sequence bigserial PRIMARY KEY,
  id uuid NOT NULL UNIQUE DEFAULT gen_random_uuid(),
  event_type text NOT NULL,
  version int NOT NULL CHECK (version >= 1),
  occurred_at timestamptz NOT NULL,
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  correlation_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  actor_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  source text NOT NULL,
  payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  schema_valid bool NOT NULL DEFAULT false,
  validation_error text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ops_event_store_org_sequence
  ON ops_event_store (org_id, sequence);

CREATE INDEX IF NOT EXISTS idx_ops_event_store_event_type_sequence
  ON ops_event_store (event_type, sequence);

CREATE TABLE IF NOT EXISTS ops_event_contracts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_type text NOT NULL,
  version int NOT NULL CHECK (version >= 1),
  schema_json jsonb NOT NULL,
  enabled bool NOT NULL DEFAULT true,
  created_by text,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (event_type, version)
);

CREATE TABLE IF NOT EXISTS ops_consumer_offsets (
  consumer_group text PRIMARY KEY,
  last_seen_sequence bigint NOT NULL DEFAULT 0 CHECK (last_seen_sequence >= 0),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ops_tenant_registry (
  org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
  tier text NOT NULL DEFAULT 'starter' CHECK (tier IN ('starter', 'growth', 'enterprise')),
  max_ttl_seconds int NOT NULL DEFAULT 1800 CHECK (max_ttl_seconds >= 60),
  auto_mitigate_enabled bool NOT NULL DEFAULT false,
  cost_model_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ops_tenant_contacts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  name text NOT NULL,
  channel text NOT NULL CHECK (channel IN ('email', 'slack', 'pagerduty', 'sms')),
  destination text NOT NULL,
  severity_min text NOT NULL DEFAULT 'MED' CHECK (severity_min IN ('LOW', 'MED', 'HIGH', 'CRIT')),
  is_primary bool NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ops_tenant_contacts_org_created_at
  ON ops_tenant_contacts (org_id, created_at DESC);

CREATE TABLE IF NOT EXISTS ops_incident_settings (
  org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
  auto_open_threshold_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  cooldown_seconds int NOT NULL DEFAULT 300 CHECK (cooldown_seconds >= 0),
  monitoring_window_seconds int NOT NULL DEFAULT 600 CHECK (monitoring_window_seconds >= 60),
  external_comms_requires_approval bool NOT NULL DEFAULT true,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ops_knowledge_docs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  doc_type text NOT NULL CHECK (doc_type IN ('runbook', 'postmortem', 'policy_diff')),
  title text NOT NULL,
  body_md text NOT NULL,
  tags_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  source_ref text,
  created_by text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ops_knowledge_docs_org_type_created
  ON ops_knowledge_docs (org_id, doc_type, created_at DESC);

CREATE TABLE IF NOT EXISTS ops_action_catalog (
  action_type text PRIMARY KEY,
  schema_json jsonb NOT NULL,
  requires_approval bool NOT NULL DEFAULT false,
  max_ttl_seconds int NOT NULL DEFAULT 1800 CHECK (max_ttl_seconds >= 60),
  enabled bool NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ops_action_proposals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  incident_id uuid,
  action_type text NOT NULL REFERENCES ops_action_catalog(action_type),
  scope_json jsonb NOT NULL,
  params_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  ttl_seconds int NOT NULL CHECK (ttl_seconds >= 0),
  evidence_refs_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  idempotency_key text NOT NULL,
  status text NOT NULL CHECK (status IN ('proposed', 'dry_run_ok', 'dry_run_failed', 'awaiting_approval', 'applied', 'failed', 'rolled_back')),
  approval_required bool NOT NULL DEFAULT false,
  proposed_by text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_ops_action_proposals_org_created
  ON ops_action_proposals (org_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_action_proposals_incident
  ON ops_action_proposals (incident_id, created_at DESC);

CREATE TABLE IF NOT EXISTS ops_action_executions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  proposal_id uuid NOT NULL REFERENCES ops_action_proposals(id) ON DELETE CASCADE,
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  mode text NOT NULL CHECK (mode IN ('dry_run', 'apply', 'rollback')),
  status text NOT NULL CHECK (status IN ('ok', 'failed')),
  error_code text,
  error_message text,
  output_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  executed_by text,
  executed_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ops_action_executions_org_executed
  ON ops_action_executions (org_id, executed_at DESC);

CREATE TABLE IF NOT EXISTS ops_action_approvals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  proposal_id uuid NOT NULL REFERENCES ops_action_proposals(id) ON DELETE CASCADE,
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  approved bool NOT NULL,
  approver text NOT NULL,
  comment text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ops_action_approvals_proposal_created
  ON ops_action_approvals (proposal_id, created_at DESC);

CREATE TABLE IF NOT EXISTS ops_diagnosis_reports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  incident_id uuid,
  provider text NOT NULL,
  model text NOT NULL,
  status text NOT NULL CHECK (status IN ('valid', 'invalid')),
  report_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  validation_error text,
  created_by text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ops_diagnosis_reports_org_created
  ON ops_diagnosis_reports (org_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_diagnosis_reports_incident
  ON ops_diagnosis_reports (incident_id, created_at DESC);

CREATE TABLE IF NOT EXISTS ops_comms_drafts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  incident_id uuid,
  channel text NOT NULL CHECK (channel IN ('internal', 'external', 'status_page')),
  audience text NOT NULL,
  status text NOT NULL CHECK (status IN ('draft', 'awaiting_approval', 'sent_internal', 'sent_external')),
  content_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  requires_approval bool NOT NULL DEFAULT false,
  created_by text,
  created_at timestamptz NOT NULL DEFAULT now(),
  sent_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_ops_comms_drafts_org_created
  ON ops_comms_drafts (org_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ops_comms_drafts_incident
  ON ops_comms_drafts (incident_id, created_at DESC);

CREATE TABLE IF NOT EXISTS ops_incident_fingerprints (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  fingerprint text NOT NULL,
  incident_id uuid,
  state text NOT NULL DEFAULT 'open',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, fingerprint)
);

CREATE TABLE IF NOT EXISTS ops_sentry_baselines (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  tool_name text NOT NULL,
  metric text NOT NULL,
  ewma_value double precision NOT NULL DEFAULT 0,
  sample_count bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, tool_name, metric)
);

DROP TRIGGER IF EXISTS trg_ops_tenant_registry_set_updated_at ON ops_tenant_registry;
CREATE TRIGGER trg_ops_tenant_registry_set_updated_at
BEFORE UPDATE ON ops_tenant_registry
FOR EACH ROW
EXECUTE PROCEDURE set_updated_at();

DROP TRIGGER IF EXISTS trg_ops_incident_settings_set_updated_at ON ops_incident_settings;
CREATE TRIGGER trg_ops_incident_settings_set_updated_at
BEFORE UPDATE ON ops_incident_settings
FOR EACH ROW
EXECUTE PROCEDURE set_updated_at();

DROP TRIGGER IF EXISTS trg_ops_knowledge_docs_set_updated_at ON ops_knowledge_docs;
CREATE TRIGGER trg_ops_knowledge_docs_set_updated_at
BEFORE UPDATE ON ops_knowledge_docs
FOR EACH ROW
EXECUTE PROCEDURE set_updated_at();

DROP TRIGGER IF EXISTS trg_ops_action_catalog_set_updated_at ON ops_action_catalog;
CREATE TRIGGER trg_ops_action_catalog_set_updated_at
BEFORE UPDATE ON ops_action_catalog
FOR EACH ROW
EXECUTE PROCEDURE set_updated_at();

DROP TRIGGER IF EXISTS trg_ops_action_proposals_set_updated_at ON ops_action_proposals;
CREATE TRIGGER trg_ops_action_proposals_set_updated_at
BEFORE UPDATE ON ops_action_proposals
FOR EACH ROW
EXECUTE PROCEDURE set_updated_at();

DROP TRIGGER IF EXISTS trg_ops_incident_fingerprints_set_updated_at ON ops_incident_fingerprints;
CREATE TRIGGER trg_ops_incident_fingerprints_set_updated_at
BEFORE UPDATE ON ops_incident_fingerprints
FOR EACH ROW
EXECUTE PROCEDURE set_updated_at();
