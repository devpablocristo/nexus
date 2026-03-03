CREATE TABLE IF NOT EXISTS pending_approvals (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES orgs(id),
    tool_id         UUID NOT NULL REFERENCES tools(id),
    request_id      TEXT NOT NULL,
    tool_name       TEXT NOT NULL,
    actor           TEXT,
    role            TEXT,
    input_redacted  JSONB NOT NULL DEFAULT '{}'::jsonb,
    context_redacted JSONB NOT NULL DEFAULT '{}'::jsonb,
    reason          TEXT NOT NULL DEFAULT '',
    policy_id       UUID,
    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'approved', 'rejected', 'expired')),
    decided_by      TEXT,
    decided_at      TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_pending_approvals_org_status ON pending_approvals(org_id, status);
CREATE INDEX idx_pending_approvals_expires ON pending_approvals(expires_at) WHERE status = 'pending';
