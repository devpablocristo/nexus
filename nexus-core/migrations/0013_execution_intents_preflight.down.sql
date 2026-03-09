ALTER TABLE execution_intents
    DROP COLUMN IF EXISTS preflight_completed_at;

ALTER TABLE execution_intents
    DROP COLUMN IF EXISTS preflight_artifact_sha256;

ALTER TABLE execution_intents
    DROP COLUMN IF EXISTS preflight_summary;

ALTER TABLE execution_intents
    DROP COLUMN IF EXISTS preflight_status;
