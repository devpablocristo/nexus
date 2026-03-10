CREATE TABLE IF NOT EXISTS orgs (
    id uuid PRIMARY KEY,
    name text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS org_api_keys (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    api_key_hash text NOT NULL UNIQUE,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS org_api_key_scopes (
    id uuid PRIMARY KEY,
    api_key_id uuid NOT NULL REFERENCES org_api_keys(id) ON DELETE CASCADE,
    scope text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_org_api_key_scopes_api_key_id ON org_api_key_scopes(api_key_id);

CREATE TABLE IF NOT EXISTS tenant_settings (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    plan_code text NOT NULL,
    hard_limits_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_by text NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS admin_activity_events (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor text NULL,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text NULL,
    payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_admin_activity_events_org_created ON admin_activity_events(org_id, created_at DESC);

CREATE TABLE IF NOT EXISTS org_usage_counters (
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    period date NOT NULL,
    counter text NOT NULL,
    value bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, period, counter)
);

CREATE TABLE IF NOT EXISTS saas_usage_event_dedup (
    event_id text PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    counter text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
