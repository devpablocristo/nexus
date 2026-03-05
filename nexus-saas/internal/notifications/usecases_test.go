package notifications

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	notificationdomain "nexus-saas/internal/notifications/usecases/domain"
)

type sentMail struct {
	To       string
	Subject  string
	HTMLBody string
	TextBody string
}

type fakeEmailSender struct {
	sent []sentMail
	err  error
}

func (f *fakeEmailSender) Send(_ context.Context, to, subject, htmlBody, textBody string) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, sentMail{To: to, Subject: subject, HTMLBody: htmlBody, TextBody: textBody})
	return nil
}

func TestUsecases_GetAndUpdatePreferences(t *testing.T) {
	db := newNotificationsTestDB(t)
	repo := NewRepository(db)
	sender := &fakeEmailSender{}
	uc := newNotificationsUsecases(repo, sender)

	orgID, user := seedOrgAndUser(t, db, "Acme", "user_1", "user1@acme.test", "admin")
	_ = orgID

	prefs, err := uc.GetPreferences(context.Background(), user)
	if err != nil {
		t.Fatalf("GetPreferences: %v", err)
	}
	if len(prefs) != len(notificationdomain.OrderedNotificationTypes()) {
		t.Fatalf("unexpected preference count: %d", len(prefs))
	}
	for _, item := range prefs {
		if !item.Enabled {
			t.Fatalf("expected default enabled preference for %s", item.NotificationType)
		}
	}

	if err := uc.UpdatePreference(context.Background(), user, "payment_failed", false); err != nil {
		t.Fatalf("UpdatePreference: %v", err)
	}
	updated, err := uc.GetPreferencesByExternalID(context.Background(), "user_1")
	if err != nil {
		t.Fatalf("GetPreferencesByExternalID: %v", err)
	}
	foundDisabled := false
	for _, item := range updated {
		if item.NotificationType == notificationdomain.NotificationPaymentFailed {
			foundDisabled = !item.Enabled
		}
	}
	if !foundDisabled {
		t.Fatalf("expected payment_failed preference to be disabled")
	}
}

func TestUsecases_NotifyPlanUpgradedTargetsAdminsAndDeduplicates(t *testing.T) {
	db := newNotificationsTestDB(t)
	repo := NewRepository(db)
	sender := &fakeEmailSender{}
	uc := newNotificationsUsecases(repo, sender)

	orgID, adminUser := seedOrgAndUser(t, db, "Nexus", "admin_1", "admin@nexus.test", "admin")
	_, secopsUser := seedMemberOnly(t, db, orgID, "secops_1", "secops@nexus.test", "secops")

	if err := uc.UpdatePreference(context.Background(), secopsUser, "plan_upgraded", false); err != nil {
		t.Fatalf("disable secops preference: %v", err)
	}

	notifData := map[string]string{
		"plan_code":    "growth",
		"org_name":     "Nexus",
		"reference_id": "sub_001",
	}
	if err := uc.Notify(context.Background(), orgID, "plan_upgraded", notifData); err != nil {
		t.Fatalf("Notify(plan_upgraded): %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 email, got %d", len(sender.sent))
	}
	if sender.sent[0].To != "admin@nexus.test" {
		t.Fatalf("unexpected recipient: %s", sender.sent[0].To)
	}
	if !strings.Contains(strings.ToLower(sender.sent[0].Subject), "upgraded") {
		t.Fatalf("unexpected subject: %s", sender.sent[0].Subject)
	}
	if count := countNotificationLogs(t, db); count != 1 {
		t.Fatalf("expected 1 notification_log row, got %d", count)
	}

	if err := uc.Notify(context.Background(), orgID, "plan_upgraded", notifData); err != nil {
		t.Fatalf("Notify(plan_upgraded dedup): %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected dedup to suppress second send, got %d", len(sender.sent))
	}
	if count := countNotificationLogs(t, db); count != 1 {
		t.Fatalf("expected dedup to suppress second log, got %d", count)
	}

	_ = adminUser
}

func TestUsecases_NotifyIncidentOpenedTargetsAllMembers(t *testing.T) {
	db := newNotificationsTestDB(t)
	repo := NewRepository(db)
	sender := &fakeEmailSender{}
	uc := newNotificationsUsecases(repo, sender)

	orgID, _ := seedOrgAndUser(t, db, "Orbit", "admin_2", "admin@orbit.test", "admin")
	_, _ = seedMemberOnly(t, db, orgID, "secops_2", "secops@orbit.test", "secops")

	if err := uc.Notify(context.Background(), orgID, "incident_opened", map[string]string{
		"org_name":          "Orbit",
		"incident_title":    "Gateway latency spike",
		"incident_severity": "high",
		"reference_id":      "inc_101",
	}); err != nil {
		t.Fatalf("Notify(incident_opened): %v", err)
	}
	if len(sender.sent) != 2 {
		t.Fatalf("expected 2 incident emails, got %d", len(sender.sent))
	}
	if !strings.Contains(sender.sent[0].HTMLBody, "Manage preferences") {
		t.Fatalf("expected footer preferences link")
	}
}

func TestUsecases_NotifyUserWelcome(t *testing.T) {
	db := newNotificationsTestDB(t)
	repo := NewRepository(db)
	sender := &fakeEmailSender{}
	uc := newNotificationsUsecases(repo, sender)

	orgID, _ := seedOrgAndUser(t, db, "Helios", "user_welcome", "welcome@helios.test", "admin")

	if err := uc.NotifyUser(context.Background(), "user_welcome", "welcome", map[string]string{
		"org_id":         orgID.String(),
		"org_name":       "Helios",
		"recipient_name": "Welcome User",
	}); err != nil {
		t.Fatalf("NotifyUser(welcome): %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 welcome email, got %d", len(sender.sent))
	}
	if sender.sent[0].Subject != "Welcome to Nexus" {
		t.Fatalf("unexpected welcome subject: %s", sender.sent[0].Subject)
	}
	if !strings.Contains(sender.sent[0].TextBody, "Manage preferences") {
		t.Fatalf("expected preferences URL in text body")
	}
}

func newNotificationsUsecases(repo *Repository, sender EmailSender) *Usecases {
	fixedTime := time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC)
	return &Usecases{
		repo:             repo,
		sender:           sender,
		logger:           zerolog.Nop(),
		towerBaseURL:     "http://localhost:5173",
		preferencesURL:   "http://localhost:5173/settings/notifications",
		defaultActionURL: "http://localhost:5173/tools",
		now:              func() time.Time { return fixedTime },
	}
}

func newNotificationsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	stmts := []string{
		`CREATE TABLE orgs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			external_id TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			avatar_url TEXT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE org_members (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL,
			joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, user_id)
		)`,
		`CREATE TABLE notification_preferences (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			notification_type TEXT NOT NULL,
			channel TEXT NOT NULL DEFAULT 'email',
			enabled BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, notification_type, channel)
		)`,
		`CREATE TABLE notification_log (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL,
			user_id TEXT NULL,
			notification_type TEXT NOT NULL,
			channel TEXT NOT NULL DEFAULT 'email',
			recipient TEXT NOT NULL,
			subject TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'sent',
			dedup_key TEXT NULL UNIQUE,
			error_message TEXT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

func seedOrgAndUser(t *testing.T, db *gorm.DB, orgName, externalID, email, role string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	orgID := uuid.New()
	if err := db.Exec(`INSERT INTO orgs(id,name,created_at) VALUES (?,?,CURRENT_TIMESTAMP)`, orgID.String(), orgName).Error; err != nil {
		t.Fatalf("seed org: %v", err)
	}
	_, userID := seedMemberOnly(t, db, orgID, externalID, email, role)
	return orgID, userID
}

func seedMemberOnly(t *testing.T, db *gorm.DB, orgID uuid.UUID, externalID, email, role string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	userID := uuid.New()
	memberID := uuid.New()
	if err := db.Exec(`INSERT INTO users(id,external_id,email,name,created_at,updated_at) VALUES (?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, userID.String(), externalID, email, externalID).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.Exec(`INSERT INTO org_members(id,org_id,user_id,role,joined_at) VALUES (?,?,?,?,CURRENT_TIMESTAMP)`, memberID.String(), orgID.String(), userID.String(), role).Error; err != nil {
		t.Fatalf("seed org member: %v", err)
	}
	return memberID, userID
}

func countNotificationLogs(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Table("notification_log").Count(&count).Error; err != nil {
		t.Fatalf("count notification_log: %v", err)
	}
	return count
}
