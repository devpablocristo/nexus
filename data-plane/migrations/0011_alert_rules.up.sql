CREATE TABLE IF NOT EXISTS alert_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES orgs(id),
    name            TEXT NOT NULL,
    metric          TEXT NOT NULL CHECK (metric IN ('deny_rate', 'error_rate', 'latency_p95', 'rate_limited_count')),
    threshold       DOUBLE PRECISION NOT NULL,
    window_seconds  INT NOT NULL DEFAULT 300,
    tool_name       TEXT,
    webhook_url     TEXT NOT NULL,
    cooldown_seconds INT NOT NULL DEFAULT 300,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    last_fired_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(org_id, name)
);
