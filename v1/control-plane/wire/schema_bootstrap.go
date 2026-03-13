package wire

import (
	actionmodels "control-plane/internal/actions/repository/models"
	alertmodels "control-plane/internal/alerts/repository/models"
	eventmodels "control-plane/internal/events/repository/models"
	incidentmodels "control-plane/internal/incidents/repository/models"
	policymodels "control-plane/internal/policyproposal/repository/models"
	sessionmodels "control-plane/internal/session/repository/models"

	"gorm.io/gorm"
)

// ensureSaaSSchema bootstraps the minimum schema required by nexus-saas.
// It is idempotent (CREATE TABLE IF NOT EXISTS) and safe to run on startup.
// Tables defined here intentionally duplicate the SQL migration files so the
// service can self-heal when running without a migration step (e.g. dev, tests).
func ensureSaaSSchema(db *gorm.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS orgs (
			id uuid PRIMARY KEY,
			name text NOT NULL UNIQUE,
			created_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS org_api_keys (
			id uuid PRIMARY KEY,
			org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
			api_key_hash text NOT NULL UNIQUE,
			name text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS org_api_key_scopes (
			id uuid PRIMARY KEY,
			api_key_id uuid NOT NULL REFERENCES org_api_keys(id) ON DELETE CASCADE,
			scope text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_org_api_key_scopes_api_key_id ON org_api_key_scopes(api_key_id)`,
		`CREATE TABLE IF NOT EXISTS tenant_settings (
			org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
			plan_code text NOT NULL,
			hard_limits_json jsonb NOT NULL DEFAULT '{}'::jsonb,
			stripe_customer_id text UNIQUE,
			stripe_subscription_id text UNIQUE,
			billing_status text NOT NULL DEFAULT 'trialing'
				CHECK (billing_status IN ('trialing','active','past_due','canceled','unpaid')),
			updated_by text NULL,
			updated_at timestamptz NOT NULL DEFAULT now(),
			created_at timestamptz NOT NULL DEFAULT now()
		)`,
		`ALTER TABLE tenant_settings ADD COLUMN IF NOT EXISTS stripe_customer_id text UNIQUE`,
		`ALTER TABLE tenant_settings ADD COLUMN IF NOT EXISTS stripe_subscription_id text UNIQUE`,
		`ALTER TABLE tenant_settings ADD COLUMN IF NOT EXISTS billing_status text NOT NULL DEFAULT 'trialing'
			CHECK (billing_status IN ('trialing','active','past_due','canceled','unpaid'))`,
		`ALTER TABLE tenant_settings ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active'
			CHECK (status IN ('active','suspended','deleted'))`,
		`ALTER TABLE tenant_settings ADD COLUMN IF NOT EXISTS deleted_at timestamptz`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_settings_stripe_customer
			ON tenant_settings(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL`,
		`CREATE TABLE IF NOT EXISTS admin_activity_events (
			id uuid PRIMARY KEY,
			org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
			actor text NULL,
			action text NOT NULL,
			resource_type text NOT NULL,
			resource_id text NULL,
			payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
			created_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_admin_activity_events_org_created ON admin_activity_events(org_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS org_usage_counters (
			org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
			period date NOT NULL,
			counter text NOT NULL,
			value bigint NOT NULL DEFAULT 0,
			updated_at timestamptz NOT NULL DEFAULT now(),
			PRIMARY KEY (org_id, period, counter)
		)`,
		`CREATE TABLE IF NOT EXISTS saas_usage_event_dedup (
			event_id text PRIMARY KEY,
			org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
			counter text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id uuid PRIMARY KEY,
			external_id text NOT NULL UNIQUE,
			email text NOT NULL UNIQUE,
			name text NOT NULL DEFAULT '',
			avatar_url text NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS org_members (
			id uuid PRIMARY KEY,
			org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
			user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role text NOT NULL DEFAULT 'secops',
			joined_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE(org_id, user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_org_members_org_id ON org_members(org_id)`,
		`CREATE INDEX IF NOT EXISTS idx_org_members_user_id ON org_members(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_external_id ON users(external_id)`,
		`CREATE TABLE IF NOT EXISTS notification_preferences (
			id uuid PRIMARY KEY,
			user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			notification_type text NOT NULL,
			channel text NOT NULL DEFAULT 'email',
			enabled boolean NOT NULL DEFAULT true,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE(user_id, notification_type, channel)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_prefs_user ON notification_preferences(user_id)`,
		`CREATE TABLE IF NOT EXISTS notification_log (
			id uuid PRIMARY KEY,
			org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
			user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
			notification_type text NOT NULL,
			channel text NOT NULL DEFAULT 'email',
			recipient text NOT NULL,
			subject text NOT NULL,
			status text NOT NULL DEFAULT 'sent',
			dedup_key text NULL,
			error_message text NULL,
			created_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_log_org_created ON notification_log(org_id, created_at DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_log_dedup_key ON notification_log(dedup_key)`,
		`CREATE TABLE IF NOT EXISTS in_app_notifications (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
			actor_id text NOT NULL DEFAULT '',
			type text NOT NULL,
			title text NOT NULL,
			body text NOT NULL DEFAULT '',
			read_at timestamptz NULL,
			created_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_inapp_notif_org_unread ON in_app_notifications (org_id, read_at) WHERE read_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_inapp_notif_actor_created ON in_app_notifications (actor_id, created_at DESC)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(
		&eventmodels.Event{},
		&actionmodels.Action{},
		&incidentmodels.Incident{},
		&policymodels.Proposal{},
		&policymodels.PolicyVersion{},
		&alertmodels.AlertRule{},
		&sessionmodels.AgentSession{},
	); err != nil {
		return err
	}
	return nil
}
