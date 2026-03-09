ALTER TABLE pending_approvals
    ADD COLUMN IF NOT EXISTS intent_id UUID;

CREATE INDEX IF NOT EXISTS idx_pending_approvals_intent_id
    ON pending_approvals(intent_id)
    WHERE intent_id IS NOT NULL;
