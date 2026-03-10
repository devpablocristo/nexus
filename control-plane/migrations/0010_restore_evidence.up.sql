CREATE TABLE IF NOT EXISTS restore_evidence (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    environment    text NOT NULL DEFAULT 'prod',
    system         text NOT NULL,
    status         text NOT NULL,
    snapshot_id    text NOT NULL DEFAULT '',
    restore_target text NOT NULL DEFAULT '',
    started_at     timestamptz,
    completed_at   timestamptz,
    source         text NOT NULL DEFAULT '',
    artifact_sha256 text NOT NULL DEFAULT '',
    summary_json   jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at     timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_restore_evidence_org_created
    ON restore_evidence (org_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_restore_evidence_org_system_env
    ON restore_evidence (org_id, system, environment, created_at DESC);
CREATE TABLE IF NOT EXISTS restore_evidence (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    environment     text NOT NULL,
    system          text NOT NULL,
    status          text NOT NULL,
    snapshot_id     text NOT NULL DEFAULT '',
    restore_target  text NOT NULL DEFAULT '',
    started_at      timestamptz,
    completed_at    timestamptz,
    source          text NOT NULL DEFAULT '',
    artifact_sha256 text NOT NULL DEFAULT '',
    summary_json    jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at      timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_restore_evidence_org_env_created
    ON restore_evidence (org_id, environment, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_restore_evidence_org_status_completed
    ON restore_evidence (org_id, status, completed_at DESC);
