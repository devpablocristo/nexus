CREATE TABLE IF NOT EXISTS action_policies (
	id uuid PRIMARY KEY,
	action_type text NOT NULL,
	resource_type text NOT NULL,
	effect text NOT NULL,
	priority integer NOT NULL,
	expression text NOT NULL,
	reason text NOT NULL,
	require_approval boolean NOT NULL,
	approval_ttl_seconds integer NOT NULL,
	enabled boolean NOT NULL,
	archived_at timestamptz NULL,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL
);
