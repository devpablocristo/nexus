CREATE TABLE IF NOT EXISTS audit_records (
	id uuid PRIMARY KEY,
	event_type text NOT NULL,
	source_service text NOT NULL,
	action_id text NULL,
	resource_id text NULL,
	resource_type text NULL,
	actor_type text NULL,
	actor_id text NULL,
	summary text NOT NULL,
	data jsonb NOT NULL DEFAULT '{}'::jsonb,
	occurred_at timestamptz NOT NULL,
	created_at timestamptz NOT NULL
)
