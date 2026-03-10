package domain

import "github.com/google/uuid"

type Principal struct {
	OrgID  uuid.UUID
	Actor  string
	Role   string
	Scopes []string
}
