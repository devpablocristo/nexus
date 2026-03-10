CREATE TABLE IF NOT EXISTS protected_resources (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name          text NOT NULL,
    resource_type text NOT NULL,
    match_value   text NOT NULL,
    match_mode    text NOT NULL DEFAULT 'exact',
    environment   text NOT NULL DEFAULT '*',
    reason        text NOT NULL DEFAULT '',
    enabled       boolean NOT NULL DEFAULT true,
    created_by    text,
    updated_by    text,
    created_at    timestamptz NOT NULL DEFAULT NOW(),
    updated_at    timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_protected_resources_org_created
    ON protected_resources (org_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_protected_resources_org_enabled
    ON protected_resources (org_id, enabled);
