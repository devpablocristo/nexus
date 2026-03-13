DROP INDEX IF EXISTS idx_pending_approvals_intent_id;

ALTER TABLE pending_approvals
    DROP COLUMN IF EXISTS intent_id;
