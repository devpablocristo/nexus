CREATE TABLE IF NOT EXISTS incidents (
	id uuid PRIMARY KEY,
	source_kind text NOT NULL,
	source_id text NOT NULL,
	action_type text NOT NULL,
	resource_id text NOT NULL,
	resource_type text NOT NULL,
	trigger text NOT NULL,
	risk_level text NOT NULL,
	severity text NOT NULL,
	status text NOT NULL,
	summary text NOT NULL,
	reason text NOT NULL,
	details jsonb NOT NULL DEFAULT '{}'::jsonb,
	archived_at timestamptz NULL,
	resolved_at timestamptz NULL,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL
);
