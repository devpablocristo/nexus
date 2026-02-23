package domain

import (
	"time"

	"github.com/google/uuid"
)

type DocType string

const (
	DocTypeRunbook    DocType = "runbook"
	DocTypePostmortem DocType = "postmortem"
	DocTypePolicyDiff DocType = "policy_diff"
)

type Document struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	DocType   DocType
	Title     string
	BodyMD    string
	Tags      []string
	SourceRef *string
	CreatedBy *string
	CreatedAt time.Time
	UpdatedAt time.Time
}
