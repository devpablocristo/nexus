package dto

import (
	"encoding/json"
	"time"

	"github.com/devpablocristo/nexus/v3/companion/internal/reviewclient"
	domain "github.com/devpablocristo/nexus/v3/companion/internal/tasks/usecases/domain"
)

type CreateTaskRequest struct {
	Title       string          `json:"title"`
	Goal        string          `json:"goal,omitempty"`
	Priority    string          `json:"priority,omitempty"`
	CreatedBy   string          `json:"created_by,omitempty"`
	AssignedTo  string          `json:"assigned_to,omitempty"`
	Channel     string          `json:"channel,omitempty"`
	Summary     string          `json:"summary,omitempty"`
	ContextJSON json.RawMessage `json:"context_json,omitempty"`
}

type TaskResponse struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Goal        string          `json:"goal"`
	Status      string          `json:"status"`
	Priority    string          `json:"priority"`
	CreatedBy   string          `json:"created_by"`
	AssignedTo  string          `json:"assigned_to"`
	Channel     string          `json:"channel"`
	Summary     string          `json:"summary"`
	ContextJSON json.RawMessage `json:"context_json"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	ClosedAt    *string         `json:"closed_at,omitempty"`
}

type MessageResponse struct {
	ID         string          `json:"id"`
	AuthorType string          `json:"author_type"`
	AuthorID   string          `json:"author_id"`
	Body       string          `json:"body"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	CreatedAt  string          `json:"created_at"`
}

type ActionResponse struct {
	ID               string          `json:"id"`
	ActionType       string          `json:"action_type"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	ReviewRequestID  *string         `json:"review_request_id,omitempty"`
	ErrorMessage     string          `json:"error_message,omitempty"`
	CreatedAt        string          `json:"created_at"`
}

type ArtifactResponse struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	URI       string          `json:"uri"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	CreatedAt string          `json:"created_at"`
}

type LinkedReviewRequestResponse struct {
	ActionID string                    `json:"action_id"`
	Request  *reviewclient.RequestSummary `json:"request,omitempty"`
}

type TaskDetailResponse struct {
	Task                 TaskResponse                  `json:"task"`
	Messages             []MessageResponse             `json:"messages"`
	Actions              []ActionResponse              `json:"actions"`
	Artifacts            []ArtifactResponse            `json:"artifacts"`
	LinkedReviewRequests []LinkedReviewRequestResponse `json:"linked_review_requests"`
}

type AddMessageRequest struct {
	AuthorType string `json:"author_type,omitempty"`
	AuthorID   string `json:"author_id,omitempty"`
	Body       string `json:"body"`
}

type InvestigateRequest struct {
	Note string `json:"note,omitempty"`
}

type ProposeRequest struct {
	Note           string `json:"note,omitempty"`
	TargetSystem   string `json:"target_system,omitempty"`
	TargetResource string `json:"target_resource,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
}

type ProposeResponse struct {
	Task         TaskResponse   `json:"task"`
	Action       ActionResponse `json:"action"`
	ReviewSubmit struct {
		RequestID      string `json:"request_id"`
		Decision       string `json:"decision"`
		Status         string `json:"status"`
		RiskLevel      string `json:"risk_level"`
		DecisionReason string `json:"decision_reason"`
	} `json:"review_submit"`
}

func TaskToResponse(t domain.Task) TaskResponse {
	var closed *string
	if t.ClosedAt != nil {
		s := t.ClosedAt.UTC().Format(time.RFC3339)
		closed = &s
	}
	return TaskResponse{
		ID:          t.ID.String(),
		Title:       t.Title,
		Goal:        t.Goal,
		Status:      t.Status,
		Priority:    t.Priority,
		CreatedBy:   t.CreatedBy,
		AssignedTo:  t.AssignedTo,
		Channel:     t.Channel,
		Summary:     t.Summary,
		ContextJSON: t.ContextJSON,
		CreatedAt:   t.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   t.UpdatedAt.UTC().Format(time.RFC3339),
		ClosedAt:    closed,
	}
}

func MessageToResponse(m domain.TaskMessage) MessageResponse {
	return MessageResponse{
		ID:         m.ID.String(),
		AuthorType: m.AuthorType,
		AuthorID:   m.AuthorID,
		Body:       m.Body,
		Metadata:   m.Metadata,
		CreatedAt:  m.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func ActionToResponse(a domain.TaskAction) ActionResponse {
	var rid *string
	if a.ReviewRequestID != nil {
		s := a.ReviewRequestID.String()
		rid = &s
	}
	return ActionResponse{
		ID:              a.ID.String(),
		ActionType:      a.ActionType,
		Payload:         a.Payload,
		ReviewRequestID: rid,
		ErrorMessage:    a.ErrorMessage,
		CreatedAt:       a.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func ArtifactToResponse(a domain.TaskArtifact) ArtifactResponse {
	return ArtifactResponse{
		ID:        a.ID.String(),
		Kind:      a.Kind,
		URI:       a.URI,
		Payload:   a.Payload,
		CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339),
	}
}
