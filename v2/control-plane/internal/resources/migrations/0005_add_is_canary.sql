ALTER TABLE protected_resources
	ADD COLUMN IF NOT EXISTS is_canary boolean NOT NULL DEFAULT false;
