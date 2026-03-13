CREATE TABLE IF NOT EXISTS agent_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES orgs(id),
    session_id      TEXT NOT NULL,
    actor           TEXT,
    total_calls     INT NOT NULL DEFAULT 0,
    total_writes    INT NOT NULL DEFAULT 0,
    total_denials   INT NOT NULL DEFAULT 0,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_call_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(org_id, session_id)
);

CREATE INDEX idx_agent_sessions_org ON agent_sessions(org_id);
CREATE INDEX idx_agent_sessions_last_call ON agent_sessions(last_call_at);
