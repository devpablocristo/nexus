CREATE TABLE IF NOT EXISTS alerts (
	id uuid PRIMARY KEY,
	source_kind text NOT NULL,
	source_id text NOT NULL,
	channel text NOT NULL,
	route text NOT NULL,
	severity text NOT NULL,
	status text NOT NULL,
	summary text NOT NULL,
	body text NOT NULL,
	details jsonb NOT NULL DEFAULT '{}'::jsonb,
	archived_at timestamptz NULL,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL
);
