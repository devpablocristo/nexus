CREATE TABLE IF NOT EXISTS protected_resources (
	id uuid PRIMARY KEY,
	type text NOT NULL,
	name text NOT NULL,
	environment text NOT NULL,
	chain text NOT NULL,
	labels jsonb NOT NULL DEFAULT '{}'::jsonb,
	criticality text NOT NULL,
	archived_at timestamptz NULL,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL
);
