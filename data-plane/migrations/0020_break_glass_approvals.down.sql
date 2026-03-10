DROP INDEX IF EXISTS idx_pending_approvals_group_id;
DROP INDEX IF EXISTS idx_pending_approvals_intent_status;

ALTER TABLE pending_approvals
    DROP COLUMN IF EXISTS approval_steps_total,
    DROP COLUMN IF EXISTS approval_step,
    DROP COLUMN IF EXISTS approval_group_id,
    DROP COLUMN IF EXISTS approval_mode;
