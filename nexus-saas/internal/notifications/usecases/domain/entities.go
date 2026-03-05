package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type NotificationType string

const (
	NotificationWelcome            NotificationType = "welcome"
	NotificationPlanUpgraded       NotificationType = "plan_upgraded"
	NotificationPaymentFailed      NotificationType = "payment_failed"
	NotificationSubscriptionCancel NotificationType = "subscription_canceled"
	NotificationIncidentOpened     NotificationType = "incident_opened"
	NotificationIncidentClosed     NotificationType = "incident_closed"
)

const ChannelEmail = "email"

var orderedNotificationTypes = []NotificationType{
	NotificationWelcome,
	NotificationPlanUpgraded,
	NotificationPaymentFailed,
	NotificationSubscriptionCancel,
	NotificationIncidentOpened,
	NotificationIncidentClosed,
}

func OrderedNotificationTypes() []NotificationType {
	out := make([]NotificationType, 0, len(orderedNotificationTypes))
	out = append(out, orderedNotificationTypes...)
	return out
}

func ParseNotificationType(raw string) (NotificationType, bool) {
	switch NotificationType(strings.TrimSpace(strings.ToLower(raw))) {
	case NotificationWelcome:
		return NotificationWelcome, true
	case NotificationPlanUpgraded:
		return NotificationPlanUpgraded, true
	case NotificationPaymentFailed:
		return NotificationPaymentFailed, true
	case NotificationSubscriptionCancel:
		return NotificationSubscriptionCancel, true
	case NotificationIncidentOpened:
		return NotificationIncidentOpened, true
	case NotificationIncidentClosed:
		return NotificationIncidentClosed, true
	default:
		return "", false
	}
}

func IsOrgWideNotification(t NotificationType) bool {
	switch t {
	case NotificationPlanUpgraded, NotificationPaymentFailed, NotificationSubscriptionCancel, NotificationIncidentOpened, NotificationIncidentClosed:
		return true
	default:
		return false
	}
}

func IsIncidentNotification(t NotificationType) bool {
	return t == NotificationIncidentOpened || t == NotificationIncidentClosed
}

type Preference struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	NotificationType NotificationType
	Channel          string
	Enabled          bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Recipient struct {
	UserID     uuid.UUID
	ExternalID string
	Email      string
	Name       string
	Role       string
}

type TemplateData struct {
	RecipientName    string
	OrgName          string
	PlanCode         string
	IncidentTitle    string
	IncidentSeverity string
	ActionURL        string
	PreferencesURL   string
	Extra            map[string]string
}

type LogEntry struct {
	ID               uuid.UUID
	OrgID            uuid.UUID
	UserID           *uuid.UUID
	NotificationType NotificationType
	Channel          string
	Recipient        string
	Subject          string
	Status           string
	DedupKey         *string
	ErrorMessage     *string
	CreatedAt        time.Time
}
