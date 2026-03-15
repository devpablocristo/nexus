CREATE INDEX IF NOT EXISTS idx_action_policies_match
	ON action_policies (action_type, resource_type, archived_at, priority, created_at, id);
