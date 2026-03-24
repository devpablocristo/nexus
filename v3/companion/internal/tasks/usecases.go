package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/governance/go/reviewclient"
	domain "github.com/devpablocristo/nexus/v3/companion/internal/tasks/usecases/domain"
)

// Identidad del servicio Companion ante Review (documentado en README).
const (
	CompanionRequesterType = "service"
	CompanionRequesterID   = "nexus_companion"
	CompanionRequesterName = "Nexus Companion"
	ActionTypePropose      = "companion.propose"
	TaskActionInvestigate  = "investigate"
	TaskActionPropose      = "propose"
)

type reviewGateway interface {
	SubmitRequest(ctx context.Context, idempotencyKey string, body reviewclient.SubmitRequestBody) (reviewclient.SubmitResponse, error)
	GetRequest(ctx context.Context, id string) (reviewclient.RequestSummary, int, error)
}

// Usecases lógica de tareas e integración con Review.
type Usecases struct {
	repo   Repository
	review reviewGateway
}

func NewUsecases(repo Repository, review reviewGateway) *Usecases {
	return &Usecases{repo: repo, review: review}
}

type CreateTaskInput struct {
	Title       string
	Goal        string
	Priority    string
	CreatedBy   string
	AssignedTo  string
	Channel     string
	Summary     string
	ContextJSON json.RawMessage
}

func (u *Usecases) Create(ctx context.Context, in CreateTaskInput) (domain.Task, error) {
	if in.Title == "" {
		return domain.Task{}, fmt.Errorf("title is required")
	}
	t := domain.Task{
		Title:       in.Title,
		Goal:        in.Goal,
		Status:      domain.TaskStatusNew,
		Priority:    in.Priority,
		CreatedBy:   in.CreatedBy,
		AssignedTo:  in.AssignedTo,
		Channel:     in.Channel,
		Summary:     in.Summary,
		ContextJSON: in.ContextJSON,
	}
	if t.Priority == "" {
		t.Priority = "normal"
	}
	if len(t.ContextJSON) == 0 {
		t.ContextJSON = json.RawMessage(`{}`)
	}
	out, err := u.repo.CreateTask(ctx, t)
	if err != nil {
		return domain.Task{}, err
	}
	slog.Info("companion task created", "task_id", out.ID.String(), "title", out.Title, "created_by", out.CreatedBy)
	return out, nil
}

func (u *Usecases) List(ctx context.Context, limit int) ([]domain.Task, error) {
	return u.repo.ListTasks(ctx, limit)
}

type LinkedReviewRequest struct {
	ActionID uuid.UUID                  `json:"action_id"`
	Request  *reviewclient.RequestSummary `json:"request,omitempty"`
}

type TaskDetail struct {
	Task                   domain.Task                `json:"task"`
	Messages               []domain.TaskMessage       `json:"messages"`
	Actions                []domain.TaskAction        `json:"actions"`
	Artifacts              []domain.TaskArtifact      `json:"artifacts"`
	LinkedReviewRequests   []LinkedReviewRequest      `json:"linked_review_requests"`
}

func (u *Usecases) GetDetail(ctx context.Context, id uuid.UUID) (TaskDetail, error) {
	var out TaskDetail
	t, err := u.repo.GetTaskByID(ctx, id)
	if err != nil {
		return out, err
	}
	out.Task = t
	out.Messages, err = u.repo.ListMessagesByTaskID(ctx, id)
	if err != nil {
		return out, err
	}
	out.Actions, err = u.repo.ListActionsByTaskID(ctx, id)
	if err != nil {
		return out, err
	}
	out.Artifacts, err = u.repo.ListArtifactsByTaskID(ctx, id)
	if err != nil {
		return out, err
	}
	seen := make(map[uuid.UUID]struct{})
	for _, a := range out.Actions {
		if a.ReviewRequestID == nil {
			continue
		}
		rid := *a.ReviewRequestID
		if _, ok := seen[rid]; ok {
			continue
		}
		seen[rid] = struct{}{}
		sum, st, gErr := u.review.GetRequest(ctx, rid.String())
		lr := LinkedReviewRequest{ActionID: a.ID}
		if gErr != nil {
			slog.Error("review get request failed", "error", gErr, "request_id", rid)
			out.LinkedReviewRequests = append(out.LinkedReviewRequests, lr)
			continue
		}
		if st == 404 {
			out.LinkedReviewRequests = append(out.LinkedReviewRequests, lr)
			continue
		}
		lr.Request = &sum
		out.LinkedReviewRequests = append(out.LinkedReviewRequests, lr)
	}
	return out, nil
}

type AddMessageInput struct {
	AuthorType string
	AuthorID   string
	Body       string
}

func (u *Usecases) AddMessage(ctx context.Context, taskID uuid.UUID, in AddMessageInput) (domain.TaskMessage, error) {
	if in.Body == "" {
		return domain.TaskMessage{}, fmt.Errorf("body is required")
	}
	if _, err := u.repo.GetTaskByID(ctx, taskID); err != nil {
		return domain.TaskMessage{}, err
	}
	at := in.AuthorType
	if at == "" {
		at = "user"
	}
	return u.repo.InsertMessage(ctx, domain.TaskMessage{
		TaskID:     taskID,
		AuthorType: at,
		AuthorID:   in.AuthorID,
		Body:       in.Body,
	})
}

// ChatInput entrada para el endpoint de chat conversacional.
type ChatInput struct {
	TaskID  *uuid.UUID // nil = crear tarea nueva
	UserID  string
	Message string
	Channel string // "console", "api", etc.
}

// ChatResult resultado del chat.
type ChatResult struct {
	Task     domain.Task
	Messages []domain.TaskMessage
}

// Chat combina crear/reusar tarea + agregar mensaje del usuario.
// Es el endpoint principal para la interfaz conversacional del suscriptor.
func (u *Usecases) Chat(ctx context.Context, in ChatInput) (ChatResult, error) {
	if in.Message == "" {
		return ChatResult{}, fmt.Errorf("message is required")
	}

	var t domain.Task
	var err error

	if in.TaskID != nil {
		// Reusar tarea existente
		t, err = u.repo.GetTaskByID(ctx, *in.TaskID)
		if err != nil {
			return ChatResult{}, err
		}
	} else {
		// Crear tarea nueva con el primer mensaje como título
		title := in.Message
		if len(title) > 80 {
			title = title[:80]
		}
		channel := in.Channel
		if channel == "" {
			channel = "console"
		}
		t, err = u.repo.CreateTask(ctx, domain.Task{
			Title:     title,
			Status:    domain.TaskStatusNew,
			Priority:  "normal",
			CreatedBy: in.UserID,
			Channel:   channel,
		})
		if err != nil {
			return ChatResult{}, fmt.Errorf("create chat task: %w", err)
		}
		slog.Info("companion chat started", "task_id", t.ID.String(), "user_id", in.UserID)
	}

	// Agregar mensaje del usuario
	_, err = u.repo.InsertMessage(ctx, domain.TaskMessage{
		TaskID:     t.ID,
		AuthorType: "user",
		AuthorID:   in.UserID,
		Body:       in.Message,
	})
	if err != nil {
		return ChatResult{}, fmt.Errorf("insert chat message: %w", err)
	}

	// Devolver hilo completo
	msgs, err := u.repo.ListMessagesByTaskID(ctx, t.ID)
	if err != nil {
		return ChatResult{}, fmt.Errorf("list chat messages: %w", err)
	}

	return ChatResult{Task: t, Messages: msgs}, nil
}

type InvestigateInput struct {
	Note string
}

func (u *Usecases) applyTaskEvent(ctx context.Context, t domain.Task, event string) (domain.Task, error) {
	to, err := companionTaskMachine().Transition(t.Status, event)
	if err != nil {
		return domain.Task{}, ErrInvalidTaskState
	}
	t.Status = to
	if to == domain.TaskStatusDone || to == domain.TaskStatusFailed {
		now := time.Now().UTC()
		t.ClosedAt = &now
	}
	return u.repo.UpdateTask(ctx, t)
}

func (u *Usecases) Investigate(ctx context.Context, taskID uuid.UUID, in InvestigateInput) (domain.Task, error) {
	t, err := u.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return domain.Task{}, err
	}
	t, err = u.applyTaskEvent(ctx, t, evInvestigate)
	if err != nil {
		return domain.Task{}, err
	}
	if in.Note != "" {
		_, err = u.repo.InsertMessage(ctx, domain.TaskMessage{
			TaskID:     taskID,
			AuthorType: "system",
			AuthorID:   "nexus_companion",
			Body:       in.Note,
		})
		if err != nil {
			return domain.Task{}, err
		}
	}
	return t, nil
}

type ProposeInput struct {
	Note           string
	TargetSystem   string
	TargetResource string
	SessionID      string
}

func (u *Usecases) Propose(ctx context.Context, taskID uuid.UUID, in ProposeInput) (domain.Task, domain.TaskAction, reviewclient.SubmitResponse, error) {
	var zeroA domain.TaskAction
	var zeroSub reviewclient.SubmitResponse
	t, err := u.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return domain.Task{}, zeroA, zeroSub, err
	}
	switch t.Status {
	case domain.TaskStatusDone, domain.TaskStatusFailed:
		return domain.Task{}, zeroA, zeroSub, ErrInvalidTaskState
	case domain.TaskStatusWaitingForApproval:
		return domain.Task{}, zeroA, zeroSub, ErrInvalidTaskState
	case domain.TaskStatusNew, domain.TaskStatusInvestigating:
		// ok
	default:
		return domain.Task{}, zeroA, zeroSub, ErrInvalidTaskState
	}

	payload := map[string]any{
		"note": in.Note,
	}
	pj, _ := json.Marshal(payload)
	action, err := u.repo.InsertAction(ctx, domain.TaskAction{
		TaskID:     taskID,
		ActionType: TaskActionPropose,
		Payload:    pj,
	})
	if err != nil {
		return domain.Task{}, zeroA, zeroSub, err
	}

	nexusMeta := map[string]any{
		"origin":       "companion",
		"task_id":      taskID.String(),
		"proposed_by":  CompanionRequesterID,
		"human_owner":  t.CreatedBy,
		"action_id":    action.ID.String(),
	}
	if in.SessionID != "" {
		nexusMeta["session_id"] = in.SessionID
	}
	params := map[string]any{"nexus": nexusMeta}

	ctxJSON := map[string]any{
		"task_title": t.Title,
		"task_goal":  t.Goal,
		"note":       in.Note,
	}
	ctxStr, _ := json.Marshal(ctxJSON)

	reason := t.Title
	if in.Note != "" {
		reason = t.Title + ": " + in.Note
	}

	idem := fmt.Sprintf("companion-propose-%s", action.ID.String())
	submitBody := reviewclient.SubmitRequestBody{
		RequesterType:  CompanionRequesterType,
		RequesterID:    CompanionRequesterID,
		RequesterName:  CompanionRequesterName,
		ActionType:     ActionTypePropose,
		TargetSystem:   in.TargetSystem,
		TargetResource: in.TargetResource,
		Params:         params,
		Reason:         reason,
		Context:        string(ctxStr),
	}

	submitOut, subErr := u.review.SubmitRequest(ctx, idem, submitBody)
	if subErr != nil {
		slog.Warn("companion propose review submit failed",
			"task_id", taskID.String(),
			"action_id", action.ID.String(),
			"error", subErr,
		)
		_ = u.repo.UpdateActionReviewResult(ctx, action.ID, nil, subErr.Error())
		t2, ge := u.repo.GetTaskByID(ctx, taskID)
		if ge != nil {
			return domain.Task{}, action, zeroSub, ge
		}
		return t2, action, zeroSub, fmt.Errorf("review submit: %w", subErr)
	}
	reqUUID, perr := uuid.Parse(submitOut.RequestID)
	if perr != nil {
		_ = u.repo.UpdateActionReviewResult(ctx, action.ID, nil, "invalid request_id from review")
		return domain.Task{}, action, zeroSub, fmt.Errorf("parse request_id: %w", perr)
	}
	if err := u.repo.UpdateActionReviewResult(ctx, action.ID, &reqUUID, ""); err != nil {
		return domain.Task{}, action, zeroSub, err
	}

	ev, evErr := eventFromSubmitResponse(submitOut)
	if evErr != nil {
		slog.Error("companion propose unexpected review status",
			"task_id", taskID.String(),
			"action_id", action.ID.String(),
			"review_status", submitOut.Status,
			"error", evErr,
		)
		return domain.Task{}, action, submitOut, evErr
	}
	t, err = u.applyTaskEvent(ctx, t, ev)
	if err != nil {
		return domain.Task{}, action, submitOut, err
	}
	action.ReviewRequestID = &reqUUID
	slog.Info("companion propose submitted to review",
		"task_id", taskID.String(),
		"action_id", action.ID.String(),
		"review_request_id", reqUUID.String(),
		"review_decision", submitOut.Decision,
		"review_status", submitOut.Status,
		"task_status", t.Status,
	)
	return t, action, submitOut, nil
}

// SyncTaskReview consulta Review y aplica transición si el request ya resolvió (tareas en espera).
func (u *Usecases) SyncTaskReview(ctx context.Context, taskID uuid.UUID) (domain.Task, error) {
	t, err := u.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return domain.Task{}, err
	}
	if t.Status != domain.TaskStatusWaitingForApproval {
		return t, nil
	}
	rid, err := u.repo.LatestProposeReviewRequestID(ctx, taskID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return t, nil
		}
		return domain.Task{}, err
	}
	sum, st, gErr := u.review.GetRequest(ctx, rid.String())
	if gErr != nil {
		return domain.Task{}, fmt.Errorf("review get request: %w", gErr)
	}
	if st == http.StatusNotFound {
		return t, nil
	}
	ev, apply := eventFromReviewRequestStatus(sum.Status)
	if !apply {
		return t, nil
	}
	t, err = u.applyTaskEvent(ctx, t, ev)
	if err != nil {
		return domain.Task{}, err
	}
	slog.Info("companion task synced from review",
		"task_id", taskID.String(),
		"review_request_id", rid.String(),
		"review_status", sum.Status,
		"task_status", t.Status,
	)
	return t, nil
}

// SyncPendingReviewTasks sincroniza un lote de tareas en waiting_for_approval.
func (u *Usecases) SyncPendingReviewTasks(ctx context.Context, limit int) {
	if limit <= 0 {
		limit = 50
	}
	list, err := u.repo.ListTasksByStatus(ctx, domain.TaskStatusWaitingForApproval, limit)
	if err != nil {
		slog.Error("companion sync list waiting tasks", "error", err)
		return
	}
	for _, item := range list {
		if _, sErr := u.SyncTaskReview(ctx, item.ID); sErr != nil {
			slog.Warn("companion sync task failed", "task_id", item.ID.String(), "error", sErr)
		}
	}
}

// RunReviewSyncLoop ejecuta SyncPendingReviewTasks periódicamente hasta que ctx termina.
func (u *Usecases) RunReviewSyncLoop(ctx context.Context, interval time.Duration, batch int) {
	if interval <= 0 || batch <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			u.SyncPendingReviewTasks(runCtx, batch)
			cancel()
		}
	}
}

// ErrInvalidStatus para handlers.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsInvalidTaskState indica conflicto de estado (FSM / reglas de negocio).
func IsInvalidTaskState(err error) bool {
	return errors.Is(err, ErrInvalidTaskState)
}
