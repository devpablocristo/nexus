ALTER TABLE pending_approvals
    ADD COLUMN IF NOT EXISTS approval_mode TEXT NOT NULL DEFAULT 'standard',
    ADD COLUMN IF NOT EXISTS approval_group_id UUID,
    ADD COLUMN IF NOT EXISTS approval_step INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS approval_steps_total INTEGER NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_pending_approvals_intent_status
    ON pending_approvals(org_id, intent_id, status)
    WHERE intent_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_pending_approvals_group_id
    ON pending_approvals(approval_group_id)
    WHERE approval_group_id IS NOT NULL;
