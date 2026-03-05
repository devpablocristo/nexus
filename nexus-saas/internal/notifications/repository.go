package notifications

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"nexus-saas/internal/notifications/repository/models"
	notificationdomain "nexus-saas/internal/notifications/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindUserByExternalID(ctx context.Context, externalID string) (notificationdomain.Recipient, bool, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return notificationdomain.Recipient{}, false, nil
	}
	var row struct {
		ID         uuid.UUID `gorm:"column:id"`
		ExternalID string    `gorm:"column:external_id"`
		Email      string    `gorm:"column:email"`
		Name       string    `gorm:"column:name"`
	}
	err := r.db.WithContext(ctx).
		Table("users").
		Select("id, external_id, email, name").
		Where("external_id = ?", externalID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return notificationdomain.Recipient{}, false, nil
		}
		return notificationdomain.Recipient{}, false, err
	}
	return notificationdomain.Recipient{
		UserID:     row.ID,
		ExternalID: row.ExternalID,
		Email:      row.Email,
		Name:       row.Name,
	}, true, nil
}

func (r *Repository) FindUserByID(ctx context.Context, userID uuid.UUID) (notificationdomain.Recipient, bool, error) {
	if userID == uuid.Nil {
		return notificationdomain.Recipient{}, false, nil
	}
	var row struct {
		ID         uuid.UUID `gorm:"column:id"`
		ExternalID string    `gorm:"column:external_id"`
		Email      string    `gorm:"column:email"`
		Name       string    `gorm:"column:name"`
	}
	err := r.db.WithContext(ctx).
		Table("users").
		Select("id, external_id, email, name").
		Where("id = ?", userID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return notificationdomain.Recipient{}, false, nil
		}
		return notificationdomain.Recipient{}, false, err
	}
	return notificationdomain.Recipient{
		UserID:     row.ID,
		ExternalID: row.ExternalID,
		Email:      row.Email,
		Name:       row.Name,
	}, true, nil
}

func (r *Repository) ListOrgRecipients(ctx context.Context, orgID uuid.UUID, adminsOnly bool) ([]notificationdomain.Recipient, error) {
	type row struct {
		UserID     uuid.UUID `gorm:"column:user_id"`
		ExternalID string    `gorm:"column:external_id"`
		Email      string    `gorm:"column:email"`
		Name       string    `gorm:"column:name"`
		Role       string    `gorm:"column:role"`
	}
	query := r.db.WithContext(ctx).
		Table("org_members m").
		Select("m.user_id, u.external_id, u.email, u.name, m.role").
		Joins("JOIN users u ON u.id = m.user_id").
		Where("m.org_id = ?", orgID)
	if adminsOnly {
		query = query.Where("m.role = ?", "admin")
	}
	var rows []row
	if err := query.Order("m.joined_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]notificationdomain.Recipient, 0, len(rows))
	for _, item := range rows {
		out = append(out, notificationdomain.Recipient{
			UserID:     item.UserID,
			ExternalID: item.ExternalID,
			Email:      item.Email,
			Name:       item.Name,
			Role:       item.Role,
		})
	}
	return out, nil
}

func (r *Repository) GetOrgName(ctx context.Context, orgID uuid.UUID) (string, error) {
	var row struct {
		Name string `gorm:"column:name"`
	}
	if err := r.db.WithContext(ctx).
		Table("orgs").
		Select("name").
		Where("id = ?", orgID).
		Take(&row).Error; err != nil {
		return "", err
	}
	return row.Name, nil
}

func (r *Repository) FindAnyOrgIDByUserID(ctx context.Context, userID uuid.UUID) (uuid.UUID, bool, error) {
	if userID == uuid.Nil {
		return uuid.Nil, false, nil
	}
	var row struct {
		OrgID uuid.UUID `gorm:"column:org_id"`
	}
	err := r.db.WithContext(ctx).
		Table("org_members").
		Select("org_id").
		Where("user_id = ?", userID).
		Order("joined_at ASC").
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, err
	}
	return row.OrgID, true, nil
}

func (r *Repository) ListPreferences(ctx context.Context, userID uuid.UUID) ([]notificationdomain.Preference, error) {
	var rows []models.NotificationPreference
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("notification_type ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]notificationdomain.Preference, 0, len(rows))
	for _, item := range rows {
		nt, ok := notificationdomain.ParseNotificationType(item.NotificationType)
		if !ok {
			continue
		}
		out = append(out, notificationdomain.Preference{
			ID:               item.ID,
			UserID:           item.UserID,
			NotificationType: nt,
			Channel:          item.Channel,
			Enabled:          item.Enabled,
			CreatedAt:        item.CreatedAt,
			UpdatedAt:        item.UpdatedAt,
		})
	}
	return out, nil
}

func (r *Repository) UpsertPreference(ctx context.Context, userID uuid.UUID, notifType notificationdomain.NotificationType, enabled bool) error {
	now := time.Now().UTC()
	row := models.NotificationPreference{
		ID:               uuid.New(),
		UserID:           userID,
		NotificationType: string(notifType),
		Channel:          notificationdomain.ChannelEmail,
		Enabled:          enabled,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "notification_type"}, {Name: "channel"}},
		DoUpdates: clause.Assignments(map[string]any{
			"enabled":    enabled,
			"updated_at": now,
		}),
	}).Create(&row).Error
}

func (r *Repository) IsPreferenceEnabled(ctx context.Context, userID uuid.UUID, notifType notificationdomain.NotificationType) (bool, error) {
	var row struct {
		Enabled bool `gorm:"column:enabled"`
	}
	err := r.db.WithContext(ctx).
		Table("notification_preferences").
		Select("enabled").
		Where("user_id = ? AND notification_type = ? AND channel = ?", userID, string(notifType), notificationdomain.ChannelEmail).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return true, nil
		}
		return false, err
	}
	return row.Enabled, nil
}

func (r *Repository) HasLogByDedupKey(ctx context.Context, dedupKey string) (bool, error) {
	dedupKey = strings.TrimSpace(dedupKey)
	if dedupKey == "" {
		return false, nil
	}
	var count int64
	if err := r.db.WithContext(ctx).
		Table("notification_log").
		Where("dedup_key = ?", dedupKey).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) CreateLog(ctx context.Context, entry notificationdomain.LogEntry) (bool, error) {
	row := models.NotificationLog{
		ID:               entry.ID,
		OrgID:            entry.OrgID,
		UserID:           entry.UserID,
		NotificationType: string(entry.NotificationType),
		Channel:          entry.Channel,
		Recipient:        entry.Recipient,
		Subject:          entry.Subject,
		Status:           entry.Status,
		DedupKey:         entry.DedupKey,
		ErrorMessage:     entry.ErrorMessage,
		CreatedAt:        entry.CreatedAt,
	}
	query := r.db.WithContext(ctx)
	if row.DedupKey != nil && strings.TrimSpace(*row.DedupKey) != "" {
		query = query.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "dedup_key"}}, DoNothing: true})
	}
	tx := query.Create(&row)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *Repository) CreateInAppNotification(ctx context.Context, item notificationdomain.InAppNotification) error {
	row := models.InAppNotification{
		ID:        item.ID,
		OrgID:     item.OrgID,
		ActorID:   strings.TrimSpace(item.ActorID),
		Type:      strings.TrimSpace(item.Type),
		Title:     strings.TrimSpace(item.Title),
		Body:      strings.TrimSpace(item.Body),
		ReadAt:    item.ReadAt,
		CreatedAt: item.CreatedAt,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *Repository) ListInAppNotifications(ctx context.Context, orgID uuid.UUID, actorID string, limit, offset int) ([]notificationdomain.InAppNotification, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	actorID = strings.TrimSpace(actorID)

	var rows []models.InAppNotification
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND actor_id = ?", orgID, actorID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]notificationdomain.InAppNotification, 0, len(rows))
	for _, row := range rows {
		out = append(out, notificationdomain.InAppNotification{
			ID:        row.ID,
			OrgID:     row.OrgID,
			ActorID:   row.ActorID,
			Type:      row.Type,
			Title:     row.Title,
			Body:      row.Body,
			ReadAt:    row.ReadAt,
			CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

func (r *Repository) CountUnreadInAppNotifications(ctx context.Context, orgID uuid.UUID, actorID string) (int64, error) {
	actorID = strings.TrimSpace(actorID)
	var count int64
	if err := r.db.WithContext(ctx).
		Table("in_app_notifications").
		Where("org_id = ? AND actor_id = ? AND read_at IS NULL", orgID, actorID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) MarkInAppNotificationRead(ctx context.Context, orgID uuid.UUID, actorID string, id uuid.UUID) error {
	actorID = strings.TrimSpace(actorID)
	if id == uuid.Nil || actorID == "" {
		return nil
	}
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&models.InAppNotification{}).
		Where("id = ? AND org_id = ? AND actor_id = ? AND read_at IS NULL", id, orgID, actorID).
		Update("read_at", now).Error
}
