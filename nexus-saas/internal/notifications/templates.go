package notifications

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"

	notificationdomain "nexus-saas/internal/notifications/usecases/domain"
)

//go:embed templates/*.html
var emailTemplateFS embed.FS

var baseEmailTemplate = template.Must(template.ParseFS(emailTemplateFS, "templates/email.html"))

type renderedEmail struct {
	Subject  string
	HTMLBody string
	TextBody string
}

type emailTemplateView struct {
	Title          string
	Message        string
	ActionURL      string
	ActionLabel    string
	OrgName        string
	PreferencesURL string
}

func renderEmailTemplate(notifType notificationdomain.NotificationType, data notificationdomain.TemplateData) (renderedEmail, error) {
	subject, view, textBody := buildEmailContent(notifType, data)
	var html bytes.Buffer
	if err := baseEmailTemplate.Execute(&html, view); err != nil {
		return renderedEmail{}, fmt.Errorf("render email template: %w", err)
	}
	return renderedEmail{
		Subject:  subject,
		HTMLBody: html.String(),
		TextBody: textBody,
	}, nil
}

func buildEmailContent(notifType notificationdomain.NotificationType, data notificationdomain.TemplateData) (string, emailTemplateView, string) {
	orgName := fallback(data.OrgName, "your organization")
	prefsURL := fallback(data.PreferencesURL, "#")
	actionURL := strings.TrimSpace(data.ActionURL)
	var title, subject, message, actionLabel string

	switch notifType {
	case notificationdomain.NotificationWelcome:
		title = "Welcome to Nexus"
		subject = "Welcome to Nexus"
		message = fmt.Sprintf("Hi %s, your account is ready. You can now access your tools, incidents, and policies in Nexus.", fallback(data.RecipientName, "there"))
		actionLabel = "Open Nexus"
	case notificationdomain.NotificationPlanUpgraded:
		title = "Plan upgraded"
		subject = fmt.Sprintf("Your plan has been upgraded to %s", strings.ToUpper(fallback(data.PlanCode, "growth")))
		message = fmt.Sprintf("Your organization %s is now on the %s plan.", orgName, strings.ToUpper(fallback(data.PlanCode, "growth")))
		actionLabel = "Review billing"
	case notificationdomain.NotificationPaymentFailed:
		title = "Payment failed"
		subject = "Action required: payment failed"
		message = fmt.Sprintf("A payment attempt for %s failed. Please update payment details to avoid service interruption.", orgName)
		actionLabel = "Fix payment method"
	case notificationdomain.NotificationSubscriptionCancel:
		title = "Subscription canceled"
		subject = "Your subscription has been canceled"
		message = fmt.Sprintf("The subscription for %s has been canceled. Your tenant has been moved to Starter.", orgName)
		actionLabel = "Review subscription"
	case notificationdomain.NotificationIncidentOpened:
		title = "Incident opened"
		subject = "Incident: " + fallback(data.IncidentTitle, "New incident detected")
		message = fmt.Sprintf("A new incident was opened for %s (severity: %s).", orgName, strings.ToUpper(fallback(data.IncidentSeverity, "unknown")))
		actionLabel = "Open incidents"
	case notificationdomain.NotificationIncidentClosed:
		title = "Incident resolved"
		subject = "Incident resolved: " + fallback(data.IncidentTitle, "Incident")
		message = fmt.Sprintf("An incident has been closed for %s.", orgName)
		actionLabel = "View incident timeline"
	default:
		title = "Nexus notification"
		subject = "Nexus notification"
		message = "A new notification is available in your Nexus workspace."
		actionLabel = "Open Nexus"
	}

	view := emailTemplateView{
		Title:          title,
		Message:        message,
		ActionURL:      actionURL,
		ActionLabel:    actionLabel,
		OrgName:        orgName,
		PreferencesURL: prefsURL,
	}
	textBody := title + "\n\n" + message + "\n"
	if actionURL != "" {
		textBody += "\n" + actionLabel + ": " + actionURL + "\n"
	}
	textBody += "\nManage preferences: " + prefsURL + "\n"
	return subject, view, textBody
}

func mapToTemplateData(raw map[string]string) notificationdomain.TemplateData {
	if raw == nil {
		raw = map[string]string{}
	}
	copyMap := make(map[string]string, len(raw))
	for k, v := range raw {
		copyMap[k] = strings.TrimSpace(v)
	}
	return notificationdomain.TemplateData{
		RecipientName:    copyMap["recipient_name"],
		OrgName:          copyMap["org_name"],
		PlanCode:         copyMap["plan_code"],
		IncidentTitle:    copyMap["incident_title"],
		IncidentSeverity: copyMap["incident_severity"],
		ActionURL:        copyMap["action_url"],
		PreferencesURL:   copyMap["preferences_url"],
		Extra:            copyMap,
	}
}

func fallback(value, def string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return def
	}
	return value
}
