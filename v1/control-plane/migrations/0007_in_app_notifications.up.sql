CREATE TABLE IF NOT EXISTS in_app_notifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor_id   text NOT NULL DEFAULT '',
    type       text NOT NULL,
    title      text NOT NULL,
    body       text NOT NULL DEFAULT '',
    read_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_inapp_notif_org_unread
    ON in_app_notifications (org_id, read_at) WHERE read_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_inapp_notif_actor_created
    ON in_app_notifications (actor_id, created_at DESC);
