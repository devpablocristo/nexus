CREATE INDEX IF NOT EXISTS idx_actions_created_at
	ON actions (created_at DESC, id DESC);
