package users

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	orgmodels "nexus-saas/internal/org/repository/models"
	usermodels "nexus-saas/internal/users/repository/models"
	userdomain "nexus-saas/internal/users/usecases/domain"
	"nexus/pkg/types"
	"nexus/pkg/utils"
)

var defaultAPIKeyScopes = []string{
	"tools:read",
	"tools:write",
	"policy:read",
	"policy:write",
	"egress:read",
	"egress:write",
	"audit:read",
	"gateway:run",
	"gateway:simulate",
	"admin:secrets",
	"admin:console:read",
	"admin:console:write",
}

type Repository struct {
	db *gorm.DB
}

type CreateAPIKeyInput struct {
	Name   string
	Scopes []string
}

type CreatedAPIKey struct {
	Key userdomain.APIKey
	Raw string
}

type RotatedAPIKey struct {
	Raw       string
	RotatedAt time.Time
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) UpsertUser(
	ctx context.Context,
	externalID, email, name string,
	avatarURL *string,
) (userdomain.User, error) {
	externalID = strings.TrimSpace(externalID)
	email = strings.TrimSpace(email)
	name = strings.TrimSpace(name)
	if externalID == "" {
		return userdomain.User{}, types.NewHTTPError(400, types.ErrCodeValidation, "external_id required")
	}
	if email == "" {
		return userdomain.User{}, types.NewHTTPError(400, types.ErrCodeValidation, "email required")
	}
	if name == "" {
		name = email
	}

	var row usermodels.User
	err := r.db.WithContext(ctx).Where("external_id = ?", externalID).Take(&row).Error
	if err == nil {
		row.Email = email
		row.Name = name
		row.AvatarURL = normalizeNullableString(avatarURL)
		row.UpdatedAt = time.Now().UTC()
		if saveErr := r.db.WithContext(ctx).Save(&row).Error; saveErr != nil {
			if isUniqueViolation(saveErr) {
				return toDomainUser(row), nil
			}
			return userdomain.User{}, saveErr
		}
		return toDomainUser(row), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return userdomain.User{}, err
	}

	row = usermodels.User{
		ID:         uuid.New(),
		ExternalID: externalID,
		Email:      email,
		Name:       name,
		AvatarURL:  normalizeNullableString(avatarURL),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return userdomain.User{}, err
	}
	return toDomainUser(row), nil
}

func (r *Repository) FindUserByExternalID(ctx context.Context, externalID string) (userdomain.User, bool, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return userdomain.User{}, false, nil
	}
	var row usermodels.User
	if err := r.db.WithContext(ctx).Where("external_id = ?", externalID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return userdomain.User{}, false, nil
		}
		return userdomain.User{}, false, err
	}
	return toDomainUser(row), true, nil
}

func (r *Repository) UpsertOrgByName(ctx context.Context, name string) (uuid.UUID, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return uuid.Nil, types.NewHTTPError(400, types.ErrCodeValidation, "org name required")
	}

	var org orgmodels.Org
	err := r.db.WithContext(ctx).Where("name = ?", name).Take(&org).Error
	if err == nil {
		return org.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, err
	}

	org = orgmodels.Org{
		ID:        uuid.New(),
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&org).Error; err != nil {
		return uuid.Nil, err
	}
	return org.ID, nil
}

func (r *Repository) UpsertOrgMember(
	ctx context.Context,
	orgID uuid.UUID,
	userID uuid.UUID,
	role string,
) (userdomain.OrgMember, error) {
	role = normalizeRole(role)
	var row usermodels.OrgMember
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND user_id = ?", orgID, userID).
		Take(&row).Error
	if err == nil {
		row.Role = role
		if err := r.db.WithContext(ctx).Save(&row).Error; err != nil {
			return userdomain.OrgMember{}, err
		}
		return toDomainMember(row), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return userdomain.OrgMember{}, err
	}

	row = usermodels.OrgMember{
		ID:       uuid.New(),
		OrgID:    orgID,
		UserID:   userID,
		Role:     role,
		JoinedAt: time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return userdomain.OrgMember{}, err
	}
	return toDomainMember(row), nil
}

func (r *Repository) ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]userdomain.OrgMember, error) {
	type row struct {
		ID         uuid.UUID `gorm:"column:id"`
		OrgID      uuid.UUID `gorm:"column:org_id"`
		UserID     uuid.UUID `gorm:"column:user_id"`
		Role       string    `gorm:"column:role"`
		JoinedAt   time.Time `gorm:"column:joined_at"`
		ExternalID string    `gorm:"column:external_id"`
		Email      string    `gorm:"column:email"`
		Name       string    `gorm:"column:name"`
		AvatarURL  *string   `gorm:"column:avatar_url"`
		CreatedAt  time.Time `gorm:"column:created_at"`
		UpdatedAt  time.Time `gorm:"column:updated_at"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("org_members m").
		Select(`
			m.id, m.org_id, m.user_id, m.role, m.joined_at,
			u.external_id, u.email, u.name, u.avatar_url, u.created_at, u.updated_at
		`).
		Joins("JOIN users u ON u.id = m.user_id").
		Where("m.org_id = ?", orgID).
		Order("m.joined_at ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]userdomain.OrgMember, 0, len(rows))
	for _, item := range rows {
		out = append(out, userdomain.OrgMember{
			ID:       item.ID,
			OrgID:    item.OrgID,
			UserID:   item.UserID,
			Role:     item.Role,
			JoinedAt: item.JoinedAt,
			User: userdomain.User{
				ID:         item.UserID,
				ExternalID: item.ExternalID,
				Email:      item.Email,
				Name:       item.Name,
				AvatarURL:  item.AvatarURL,
				CreatedAt:  item.CreatedAt,
				UpdatedAt:  item.UpdatedAt,
			},
		})
	}
	return out, nil
}

func (r *Repository) ListAPIKeys(ctx context.Context, orgID uuid.UUID) ([]userdomain.APIKey, error) {
	var keys []orgmodels.OrgAPIKey
	if err := r.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Order("created_at DESC").
		Find(&keys).Error; err != nil {
		return nil, err
	}
	out := make([]userdomain.APIKey, 0, len(keys))
	for _, key := range keys {
		var scopes []string
		if err := r.db.WithContext(ctx).
			Model(&orgmodels.OrgAPIKeyScope{}).
			Where("api_key_id = ?", key.ID).
			Order("scope ASC").
			Pluck("scope", &scopes).Error; err != nil {
			return nil, err
		}
		out = append(out, userdomain.APIKey{
			ID:        key.ID,
			OrgID:     key.OrgID,
			Name:      key.Name,
			Scopes:    scopes,
			CreatedAt: key.CreatedAt,
		})
	}
	return out, nil
}

func (r *Repository) CreateAPIKey(ctx context.Context, orgID uuid.UUID, input CreateAPIKeyInput) (CreatedAPIKey, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "api-key-" + time.Now().UTC().Format("20060102150405")
	}
	scopes := normalizeScopes(input.Scopes)
	if len(scopes) == 0 {
		scopes = append([]string(nil), defaultAPIKeyScopes...)
	}

	raw := generateAPIKey()
	hash := utils.SHA256Hex(raw)
	keyRow := orgmodels.OrgAPIKey{
		ID:         uuid.New(),
		OrgID:      orgID,
		APIKeyHash: hash,
		Name:       name,
		CreatedAt:  time.Now().UTC(),
	}

	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return CreatedAPIKey{}, tx.Error
	}
	if err := tx.Create(&keyRow).Error; err != nil {
		tx.Rollback()
		return CreatedAPIKey{}, err
	}
	for _, scope := range scopes {
		scopeRow := orgmodels.OrgAPIKeyScope{
			ID:        uuid.New(),
			APIKeyID:  keyRow.ID,
			Scope:     scope,
			CreatedAt: time.Now().UTC(),
		}
		if err := tx.Create(&scopeRow).Error; err != nil {
			tx.Rollback()
			return CreatedAPIKey{}, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return CreatedAPIKey{}, err
	}

	return CreatedAPIKey{
		Key: userdomain.APIKey{
			ID:        keyRow.ID,
			OrgID:     keyRow.OrgID,
			Name:      keyRow.Name,
			Scopes:    scopes,
			CreatedAt: keyRow.CreatedAt,
		},
		Raw: raw,
	}, nil
}

func (r *Repository) DeleteAPIKey(ctx context.Context, orgID, keyID uuid.UUID) error {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	res := tx.Where("id = ? AND org_id = ?", keyID, orgID).Delete(&orgmodels.OrgAPIKey{})
	if res.Error != nil {
		tx.Rollback()
		return res.Error
	}
	if res.RowsAffected == 0 {
		tx.Rollback()
		return types.NewHTTPError(404, types.ErrCodeNotFound, "api key not found")
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	return nil
}

func (r *Repository) RotateAPIKey(ctx context.Context, orgID, keyID uuid.UUID) (RotatedAPIKey, error) {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return RotatedAPIKey{}, tx.Error
	}
	var row orgmodels.OrgAPIKey
	if err := tx.Where("id = ? AND org_id = ?", keyID, orgID).Take(&row).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RotatedAPIKey{}, types.NewHTTPError(404, types.ErrCodeNotFound, "api key not found")
		}
		return RotatedAPIKey{}, err
	}
	raw := generateAPIKey()
	row.APIKeyHash = utils.SHA256Hex(raw)
	now := time.Now().UTC()
	if err := tx.Save(&row).Error; err != nil {
		tx.Rollback()
		return RotatedAPIKey{}, err
	}
	if err := tx.Commit().Error; err != nil {
		return RotatedAPIKey{}, err
	}
	return RotatedAPIKey{Raw: raw, RotatedAt: now}, nil
}

func (r *Repository) SoftDeleteUser(ctx context.Context, externalID string) error {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return types.NewHTTPError(400, types.ErrCodeValidation, "external_id required")
	}
	var row usermodels.User
	if err := r.db.WithContext(ctx).Where("external_id = ?", externalID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	// Remove all org memberships first, then delete user
	if err := r.db.WithContext(ctx).Where("user_id = ?", row.ID).Delete(&usermodels.OrgMember{}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Delete(&row).Error
}

func (r *Repository) RemoveMembership(ctx context.Context, userExternalID, orgName string) error {
	userExternalID = strings.TrimSpace(userExternalID)
	orgName = strings.TrimSpace(orgName)
	if userExternalID == "" || orgName == "" {
		return nil
	}
	var user usermodels.User
	if err := r.db.WithContext(ctx).Where("external_id = ?", userExternalID).Take(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	var org orgmodels.Org
	if err := r.db.WithContext(ctx).Where("name = ?", orgName).Take(&org).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return r.db.WithContext(ctx).
		Where("org_id = ? AND user_id = ?", org.ID, user.ID).
		Delete(&usermodels.OrgMember{}).Error
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") || strings.Contains(msg, "23505")
}

func generateAPIKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed")
	}
	return "nxk_" + hex.EncodeToString(b)
}

func normalizeScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(scopes))
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	return out
}

func normalizeNullableString(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

func normalizeRole(role string) string {
	role = strings.TrimSpace(strings.ToLower(role))
	switch role {
	case "admin":
		return "admin"
	case "secops":
		return "secops"
	case "org:admin":
		return "admin"
	case "org:member":
		return "secops"
	default:
		return "secops"
	}
}

func toDomainUser(in usermodels.User) userdomain.User {
	return userdomain.User{
		ID:         in.ID,
		ExternalID: in.ExternalID,
		Email:      in.Email,
		Name:       in.Name,
		AvatarURL:  in.AvatarURL,
		CreatedAt:  in.CreatedAt,
		UpdatedAt:  in.UpdatedAt,
	}
}

func toDomainMember(in usermodels.OrgMember) userdomain.OrgMember {
	return userdomain.OrgMember{
		ID:       in.ID,
		OrgID:    in.OrgID,
		UserID:   in.UserID,
		Role:     in.Role,
		JoinedAt: in.JoinedAt,
	}
}

func FormatUserName(firstName, lastName, email string) string {
	firstName = strings.TrimSpace(firstName)
	lastName = strings.TrimSpace(lastName)
	email = strings.TrimSpace(email)
	switch {
	case firstName != "" && lastName != "":
		return firstName + " " + lastName
	case firstName != "":
		return firstName
	case lastName != "":
		return lastName
	case email != "":
		return email
	default:
		return "Unknown User"
	}
}

func BuildWebhookUserEmail(primaryEmail string, all []string) (string, error) {
	primaryEmail = strings.TrimSpace(primaryEmail)
	if primaryEmail != "" {
		return primaryEmail, nil
	}
	for _, email := range all {
		email = strings.TrimSpace(email)
		if email != "" {
			return email, nil
		}
	}
	return "", fmt.Errorf("email not found in webhook payload")
}
