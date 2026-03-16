ALTER TABLE action_policies
	ADD COLUMN IF NOT EXISTS is_trap boolean NOT NULL DEFAULT false;
