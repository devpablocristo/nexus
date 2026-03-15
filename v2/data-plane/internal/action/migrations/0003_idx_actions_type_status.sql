CREATE INDEX IF NOT EXISTS idx_actions_type_status
	ON actions (type, status, created_at DESC, id DESC);
