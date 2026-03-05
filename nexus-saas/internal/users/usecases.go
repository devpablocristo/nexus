package users

import (
	"context"
	"strings"

	"github.com/google/uuid"

	userdomain "nexus-saas/internal/users/usecases/domain"
	"nexus/pkg/types"
)

type Usecases struct {
	repo *Repository
}

type MeProfile struct {
	OrgID      uuid.UUID
	ExternalID string
	Role       string
	Scopes     []string
	User       *userdomain.User
}

func NewUsecases(repo *Repository) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) GetMe(ctx context.Context, orgID uuid.UUID, externalID, role string, scopes []string) (MeProfile, error) {
	me := MeProfile{
		OrgID:      orgID,
		ExternalID: strings.TrimSpace(externalID),
		Role:       strings.TrimSpace(role),
		Scopes:     scopes,
	}
	if me.ExternalID == "" {
		return me, nil
	}
	user, ok, err := u.repo.FindUserByExternalID(ctx, me.ExternalID)
	if err != nil {
		return MeProfile{}, err
	}
	if ok {
		me.User = &user
	}
	return me, nil
}

func (u *Usecases) ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]userdomain.OrgMember, error) {
	return u.repo.ListOrgMembers(ctx, orgID)
}

func (u *Usecases) ListAPIKeys(ctx context.Context, orgID uuid.UUID) ([]userdomain.APIKey, error) {
	return u.repo.ListAPIKeys(ctx, orgID)
}

func (u *Usecases) CreateAPIKey(ctx context.Context, orgID uuid.UUID, name string, scopes []string) (CreatedAPIKey, error) {
	return u.repo.CreateAPIKey(ctx, orgID, CreateAPIKeyInput{
		Name:   name,
		Scopes: scopes,
	})
}

func (u *Usecases) DeleteAPIKey(ctx context.Context, orgID, keyID uuid.UUID) error {
	return u.repo.DeleteAPIKey(ctx, orgID, keyID)
}

func (u *Usecases) RotateAPIKey(ctx context.Context, orgID, keyID uuid.UUID) (RotatedAPIKey, error) {
	return u.repo.RotateAPIKey(ctx, orgID, keyID)
}

func (u *Usecases) SyncUser(ctx context.Context, externalID, email, name string, avatarURL *string) (userdomain.User, error) {
	return u.repo.UpsertUser(ctx, externalID, email, name, avatarURL)
}

func (u *Usecases) SyncOrganization(ctx context.Context, orgName string) (uuid.UUID, error) {
	return u.repo.UpsertOrgByName(ctx, orgName)
}

func (u *Usecases) SyncMembership(
	ctx context.Context,
	orgID uuid.UUID,
	userExternalID, email, name string,
	avatarURL *string,
	role string,
) (userdomain.OrgMember, error) {
	user, err := u.repo.UpsertUser(ctx, userExternalID, email, name, avatarURL)
	if err != nil {
		return userdomain.OrgMember{}, err
	}
	member, err := u.repo.UpsertOrgMember(ctx, orgID, user.ID, role)
	if err != nil {
		return userdomain.OrgMember{}, err
	}
	member.User = user
	return member, nil
}

func (u *Usecases) SoftDeleteUser(ctx context.Context, externalID string) error {
	return u.repo.SoftDeleteUser(ctx, externalID)
}

func (u *Usecases) RemoveMembership(ctx context.Context, userExternalID, orgName string) error {
	return u.repo.RemoveMembership(ctx, userExternalID, orgName)
}

func EnsureOrgMatch(pathOrgID, tokenOrgID uuid.UUID) error {
	if pathOrgID == uuid.Nil {
		return types.NewHTTPError(400, types.ErrCodeValidation, "invalid org_id")
	}
	if tokenOrgID == uuid.Nil {
		return types.NewHTTPError(401, types.ErrCodeUnauthorized, "missing org context")
	}
	if pathOrgID != tokenOrgID {
		return types.NewHTTPError(403, types.ErrCodeUnauthorized, "cross-org access denied")
	}
	return nil
}
