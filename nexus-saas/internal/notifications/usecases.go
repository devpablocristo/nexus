package notifications

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"nexus-saas/cmd/config"
	saasmetrics "nexus-saas/internal/shared/metrics"
	notificationdomain "nexus-saas/internal/notifications/usecases/domain"
	"nexus/pkg/types"
)

type NotificationPort interface {
	Notify(ctx context.Context, orgID uuid.UUID, notifType string, data map[string]string) error
	NotifyUser(ctx context.Context, userExternalID string, notifType string, data map[string]string) error
}

type Usecases struct {
	repo             *Repository
	sender           EmailSender
	logger           zerolog.Logger
	towerBaseURL     string
	preferencesURL   string
	defaultActionURL string
	now              func() time.Time
}

func NewUsecases(cfg config.ServiceConfig, repo *Repository, sender EmailSender, logger zerolog.Logger) *Usecases {
	baseURL := sanitizeTowerBaseURL(cfg.TowerBaseURL)
	return &Usecases{
		repo:             repo,
		sender:           sender,
		logger:           logger,
		towerBaseURL:     baseURL,
		preferencesURL:   baseURL + "/settings/notifications",
		defaultActionURL: baseURL + "/tools",
		now:              time.Now,
	}
}

func (u *Usecases) Notify(ctx context.Context, orgID uuid.UUID, notifType string, data map[string]string) error {
	parsedType, ok := notificationdomain.ParseNotificationType(notifType)
	if !ok {
		return types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid notification_type")
	}
	if !notificationdomain.IsOrgWideNotification(parsedType) {
		return types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "notification type is not org-wide")
	}
	adminsOnly := !notificationdomain.IsIncidentNotification(parsedType)
	recipients, err := u.repo.ListOrgRecipients(ctx, orgID, adminsOnly)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		return nil
	}
	orgName := strings.TrimSpace(dataValue(data, "org_name"))
	if orgName == "" {
		if resolved, resolveErr := u.repo.GetOrgName(ctx, orgID); resolveErr == nil {
			orgName = resolved
		}
	}

	var firstErr error
	for _, recipient := range recipients {
		if err := u.notifyRecipient(ctx, &orgID, recipient, parsedType, mergeNotificationData(data, map[string]string{
			"org_name": orgName,
		})); err != nil {
			u.logger.Error().Err(err).Str("notification_type", string(parsedType)).Str("recipient", recipient.Email).Msg("failed sending org notification")
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (u *Usecases) NotifyUser(ctx context.Context, userExternalID string, notifType string, data map[string]string) error {
	parsedType, ok := notificationdomain.ParseNotificationType(notifType)
	if !ok {
		return types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid notification_type")
	}
	recipient, found, err := u.repo.FindUserByExternalID(ctx, userExternalID)
	if err != nil {
		return err
	}
	if !found {
		return types.NewHTTPError(http.StatusNotFound, types.ErrCodeNotFound, "user not found")
	}
	orgID := parseOptionalUUID(dataValue(data, "org_id"))
	if orgID == nil {
		if resolved, ok, err := u.repo.FindAnyOrgIDByUserID(ctx, recipient.UserID); err == nil && ok {
			orgID = &resolved
		} else if err != nil {
			u.logger.Warn().Err(err).Str("external_id", userExternalID).Msg("failed resolving org for user notification")
		}
	}
	return u.notifyRecipient(ctx, orgID, recipient, parsedType, data)
}

func (u *Usecases) GetPreferences(ctx context.Context, userID uuid.UUID) ([]notificationdomain.Preference, error) {
	if userID == uuid.Nil {
		return nil, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid user_id")
	}
	existing, err := u.repo.ListPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}
	enabledByType := make(map[notificationdomain.NotificationType]bool, len(existing))
	for _, item := range existing {
		enabledByType[item.NotificationType] = item.Enabled
	}
	orderedTypes := notificationdomain.OrderedNotificationTypes()
	out := make([]notificationdomain.Preference, 0, len(orderedTypes))
	for _, notifType := range orderedTypes {
		enabled, ok := enabledByType[notifType]
		if !ok {
			enabled = true
		}
		out = append(out, notificationdomain.Preference{
			UserID:           userID,
			NotificationType: notifType,
			Channel:          notificationdomain.ChannelEmail,
			Enabled:          enabled,
		})
	}
	return out, nil
}

func (u *Usecases) GetPreferencesByExternalID(ctx context.Context, userExternalID string) ([]notificationdomain.Preference, error) {
	recipient, found, err := u.repo.FindUserByExternalID(ctx, userExternalID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, types.NewHTTPError(http.StatusNotFound, types.ErrCodeNotFound, "user not found")
	}
	return u.GetPreferences(ctx, recipient.UserID)
}

func (u *Usecases) UpdatePreference(ctx context.Context, userID uuid.UUID, notifType string, enabled bool) error {
	if userID == uuid.Nil {
		return types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid user_id")
	}
	parsedType, ok := notificationdomain.ParseNotificationType(notifType)
	if !ok {
		return types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid notification_type")
	}
	return u.repo.UpsertPreference(ctx, userID, parsedType, enabled)
}

func (u *Usecases) UpdatePreferencesByExternalID(ctx context.Context, userExternalID string, updates map[string]bool) error {
	recipient, found, err := u.repo.FindUserByExternalID(ctx, userExternalID)
	if err != nil {
		return err
	}
	if !found {
		return types.NewHTTPError(http.StatusNotFound, types.ErrCodeNotFound, "user not found")
	}
	keys := make([]string, 0, len(updates))
	for k := range updates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, notifType := range keys {
		if err := u.UpdatePreference(ctx, recipient.UserID, notifType, updates[notifType]); err != nil {
			return err
		}
	}
	return nil
}

func (u *Usecases) notifyRecipient(
	ctx context.Context,
	orgID *uuid.UUID,
	recipient notificationdomain.Recipient,
	notifType notificationdomain.NotificationType,
	data map[string]string,
) error {
	enabled, err := u.repo.IsPreferenceEnabled(ctx, recipient.UserID, notifType)
	if err != nil {
		return err
	}
	if !enabled {
		u.logger.Debug().Str("notification_type", string(notifType)).Str("recipient", recipient.Email).Msg("notification disabled by preference")
		return nil
	}
	templateData := mapToTemplateData(mergeNotificationData(data, map[string]string{
		"recipient_name":  recipient.Name,
		"preferences_url": fallback(dataValue(data, "preferences_url"), u.preferencesURL),
		"action_url":      fallback(dataValue(data, "action_url"), u.defaultActionURL),
	}))
	rendered, err := renderEmailTemplate(notifType, templateData)
	if err != nil {
		return err
	}
	dedupKey := u.buildDedupKey(recipient.UserID, notifType, rendered.Subject, data)
	if dedupKey != nil {
		alreadySent, err := u.repo.HasLogByDedupKey(ctx, *dedupKey)
		if err != nil {
			return err
		}
		if alreadySent {
			u.logger.Debug().Str("dedup_key", *dedupKey).Str("recipient", recipient.Email).Msg("notification deduplicated")
			return nil
		}
	}

	sendErr := u.sender.Send(ctx, recipient.Email, rendered.Subject, rendered.HTMLBody, rendered.TextBody)
	status := "sent"
	var errMsg *string
	if sendErr != nil {
		status = "failed"
		v := sendErr.Error()
		errMsg = &v
	}
	if orgID != nil {
		_, logErr := u.repo.CreateLog(ctx, notificationdomain.LogEntry{
			ID:               uuid.New(),
			OrgID:            *orgID,
			UserID:           &recipient.UserID,
			NotificationType: notifType,
			Channel:          notificationdomain.ChannelEmail,
			Recipient:        recipient.Email,
			Subject:          rendered.Subject,
			Status:           status,
			DedupKey:         dedupKey,
			ErrorMessage:     errMsg,
			CreatedAt:        u.now().UTC(),
		})
		if logErr != nil {
			u.logger.Warn().Err(logErr).Str("notification_type", string(notifType)).Str("recipient", recipient.Email).Msg("failed writing notification_log")
		}
	}
	if sendErr != nil {
		return sendErr
	}
	saasmetrics.NotificationsSent.WithLabelValues(
		string(notifType),
		string(notificationdomain.ChannelEmail),
	).Inc()
	return nil
}

func (u *Usecases) buildDedupKey(userID uuid.UUID, notifType notificationdomain.NotificationType, subject string, data map[string]string) *string {
	referenceID := strings.TrimSpace(dataValue(data, "reference_id"))
	if referenceID == "" {
		referenceID = strings.TrimSpace(dataValue(data, "incident_id"))
	}
	if referenceID == "" {
		referenceID = strings.TrimSpace(dataValue(data, "subscription_id"))
	}
	if referenceID == "" {
		referenceID = strings.TrimSpace(subject)
	}
	if referenceID == "" {
		return nil
	}
	hourBucket := u.now().UTC().Format("2006010215")
	v := fmt.Sprintf("%s|%s|%s|%s", notifType, userID.String(), referenceID, hourBucket)
	return &v
}

func sanitizeTowerBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "http://localhost:5173"
	}
	return strings.TrimRight(raw, "/")
}

func dataValue(data map[string]string, key string) string {
	if data == nil {
		return ""
	}
	return strings.TrimSpace(data[key])
}

func parseOptionalUUID(raw string) *uuid.UUID {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &id
}

func mergeNotificationData(base map[string]string, overlay map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = strings.TrimSpace(v)
	}
	for k, v := range overlay {
		if strings.TrimSpace(v) == "" {
			if _, exists := out[k]; exists {
				continue
			}
		}
		out[k] = strings.TrimSpace(v)
	}
	return out
}
