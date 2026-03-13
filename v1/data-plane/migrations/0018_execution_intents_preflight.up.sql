ALTER TABLE execution_intents
    ADD COLUMN IF NOT EXISTS preflight_status TEXT NOT NULL DEFAULT 'not_required'
        CHECK (preflight_status IN ('not_required', 'passed', 'failed'));

ALTER TABLE execution_intents
    ADD COLUMN IF NOT EXISTS preflight_summary JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE execution_intents
    ADD COLUMN IF NOT EXISTS preflight_artifact_sha256 TEXT;

ALTER TABLE execution_intents
    ADD COLUMN IF NOT EXISTS preflight_completed_at TIMESTAMPTZ;
