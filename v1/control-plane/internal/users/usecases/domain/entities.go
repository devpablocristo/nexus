package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID         uuid.UUID
	ExternalID string
	Email      string
	Name       string
	AvatarURL  *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type OrgMember struct {
	ID       uuid.UUID
	OrgID    uuid.UUID
	UserID   uuid.UUID
	Role     string
	JoinedAt time.Time
	User     User
}

type APIKey struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	Name      string
	Scopes    []string
	CreatedAt time.Time
}
