CREATE TABLE IF NOT EXISTS execution_leases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL,
    intent_id uuid NOT NULL REFERENCES execution_intents(id) ON DELETE CASCADE,
    tool_name text NOT NULL,
    risk_class text NOT NULL,
    status text NOT NULL DEFAULT 'active',
    credential_mode text NOT NULL DEFAULT 'none',
    credential_hints jsonb NOT NULL DEFAULT '{}'::jsonb,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_execution_leases_org_intent
    ON execution_leases (org_id, intent_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_execution_leases_org_status_expires
    ON execution_leases (org_id, status, expires_at);
