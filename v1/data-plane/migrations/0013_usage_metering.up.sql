CREATE TABLE IF NOT EXISTS org_usage_counters (
    org_id     UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    period     DATE        NOT NULL,
    counter    TEXT        NOT NULL,
    value      BIGINT      NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, period, counter)
);

CREATE INDEX IF NOT EXISTS idx_org_usage_counters_org_period
    ON org_usage_counters (org_id, period);
