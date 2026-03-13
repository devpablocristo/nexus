CREATE TABLE IF NOT EXISTS execution_intents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES orgs(id),
    tool_id         UUID NOT NULL REFERENCES tools(id),
    tool_name       TEXT NOT NULL,
    request_id      TEXT NOT NULL,
    actor           TEXT,
    role            TEXT,
    scopes_json     JSONB NOT NULL DEFAULT '[]'::jsonb,
    input_payload   JSONB NOT NULL DEFAULT '{}'::jsonb,
    context_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    policy_id       UUID,
    risk_class      TEXT NOT NULL,
    reason          TEXT NOT NULL DEFAULT '',
    approval_id     UUID REFERENCES pending_approvals(id),
    status          TEXT NOT NULL DEFAULT 'pending_approval'
                    CHECK (status IN ('pending_approval', 'approved', 'rejected', 'executed', 'expired')),
    expires_at      TIMESTAMPTZ NOT NULL,
    approved_at     TIMESTAMPTZ,
    executed_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_execution_intents_org_status_created
    ON execution_intents(org_id, status, created_at DESC);

CREATE INDEX idx_execution_intents_approval_id
    ON execution_intents(approval_id)
    WHERE approval_id IS NOT NULL;
