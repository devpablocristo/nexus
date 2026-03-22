package tasks

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	sharedhandlers "github.com/devpablocristo/core/backend/go/httpjson"
	"github.com/google/uuid"

	"github.com/devpablocristo/nexus/v3/companion/internal/reviewclient"
	"github.com/devpablocristo/nexus/v3/companion/internal/shared"
	tasksdto "github.com/devpablocristo/nexus/v3/companion/internal/tasks/handler/dto"
	domain "github.com/devpablocristo/nexus/v3/companion/internal/tasks/usecases/domain"
)

type taskUsecase interface {
	Create(ctx context.Context, in CreateTaskInput) (domain.Task, error)
	List(ctx context.Context, limit int) ([]domain.Task, error)
	GetDetail(ctx context.Context, id uuid.UUID) (TaskDetail, error)
	AddMessage(ctx context.Context, taskID uuid.UUID, in AddMessageInput) (domain.TaskMessage, error)
	Investigate(ctx context.Context, taskID uuid.UUID, in InvestigateInput) (domain.Task, error)
	Propose(ctx context.Context, taskID uuid.UUID, in ProposeInput) (domain.Task, domain.TaskAction, reviewclient.SubmitResponse, error)
	SyncTaskReview(ctx context.Context, taskID uuid.UUID) (domain.Task, error)
}

type Handler struct {
	uc taskUsecase
}

func NewHandler(uc taskUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/tasks", h.create)
	mux.HandleFunc("GET /v1/tasks", h.list)
	mux.HandleFunc("GET /v1/tasks/{id}", h.getByID)
	mux.HandleFunc("POST /v1/tasks/{id}/message", h.addMessage)
	mux.HandleFunc("POST /v1/tasks/{id}/investigate", h.investigate)
	mux.HandleFunc("POST /v1/tasks/{id}/propose", h.propose)
	mux.HandleFunc("POST /v1/tasks/{id}/sync", h.syncReview)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body tasksdto.CreateTaskRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	if body.Title == "" {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "title is required")
		return
	}
	t, err := h.uc.Create(r.Context(), CreateTaskInput{
		Title:       body.Title,
		Goal:        body.Goal,
		Priority:    body.Priority,
		CreatedBy:   body.CreatedBy,
		AssignedTo:  body.AssignedTo,
		Channel:     body.Channel,
		Summary:     body.Summary,
		ContextJSON: body.ContextJSON,
	})
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", err.Error())
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusCreated, tasksdto.TaskToResponse(t))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit := shared.DefaultListLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= shared.MaxListLimit {
			limit = n
		}
	}
	list, err := h.uc.List(r.Context(), limit)
	if err != nil {
		shared.WriteInternalError(w, err, "list tasks failed")
		return
	}
	out := make([]tasksdto.TaskResponse, 0, len(list))
	for _, t := range list {
		out = append(out, tasksdto.TaskToResponse(t))
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	detail, err := h.uc.GetDetail(r.Context(), id)
	if err != nil {
		if IsNotFound(err) {
			shared.WriteError(w, http.StatusNotFound, "NOT_FOUND", "task not found")
			return
		}
		shared.WriteInternalError(w, err, "get task failed")
		return
	}
	resp := tasksdto.TaskDetailResponse{
		Task:      tasksdto.TaskToResponse(detail.Task),
		Messages:  make([]tasksdto.MessageResponse, 0, len(detail.Messages)),
		Actions:   make([]tasksdto.ActionResponse, 0, len(detail.Actions)),
		Artifacts: make([]tasksdto.ArtifactResponse, 0, len(detail.Artifacts)),
		LinkedReviewRequests: make([]tasksdto.LinkedReviewRequestResponse, 0, len(detail.LinkedReviewRequests)),
	}
	for _, m := range detail.Messages {
		resp.Messages = append(resp.Messages, tasksdto.MessageToResponse(m))
	}
	for _, a := range detail.Actions {
		resp.Actions = append(resp.Actions, tasksdto.ActionToResponse(a))
	}
	for _, ar := range detail.Artifacts {
		resp.Artifacts = append(resp.Artifacts, tasksdto.ArtifactToResponse(ar))
	}
	for _, lr := range detail.LinkedReviewRequests {
		resp.LinkedReviewRequests = append(resp.LinkedReviewRequests, tasksdto.LinkedReviewRequestResponse{
			ActionID: lr.ActionID.String(),
			Request:  lr.Request,
		})
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) addMessage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	var body tasksdto.AddMessageRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	m, err := h.uc.AddMessage(r.Context(), id, AddMessageInput{
		AuthorType: body.AuthorType,
		AuthorID:   body.AuthorID,
		Body:       body.Body,
	})
	if err != nil {
		if IsNotFound(err) {
			shared.WriteError(w, http.StatusNotFound, "NOT_FOUND", "task not found")
			return
		}
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", err.Error())
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusCreated, tasksdto.MessageToResponse(m))
}

func (h *Handler) investigate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	raw, _ := io.ReadAll(r.Body)
	var body tasksdto.InvestigateRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
			return
		}
	}
	t, err := h.uc.Investigate(r.Context(), id, InvestigateInput{Note: body.Note})
	if err != nil {
		if IsNotFound(err) {
			shared.WriteError(w, http.StatusNotFound, "NOT_FOUND", "task not found")
			return
		}
		if IsInvalidTaskState(err) {
			shared.WriteError(w, http.StatusConflict, "CONFLICT", "invalid task state")
			return
		}
		shared.WriteInternalError(w, err, "investigate failed")
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, tasksdto.TaskToResponse(t))
}

func (h *Handler) propose(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	var body tasksdto.ProposeRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	t, action, sub, err := h.uc.Propose(r.Context(), id, ProposeInput{
		Note:           body.Note,
		TargetSystem:   body.TargetSystem,
		TargetResource: body.TargetResource,
		SessionID:      body.SessionID,
	})
	if err != nil {
		if IsNotFound(err) {
			shared.WriteError(w, http.StatusNotFound, "NOT_FOUND", "task not found")
			return
		}
		if IsInvalidTaskState(err) {
			shared.WriteError(w, http.StatusConflict, "CONFLICT", "invalid task state")
			return
		}
		if strings.HasPrefix(err.Error(), "review submit:") && t.ID != uuid.Nil {
			sharedhandlers.WriteJSON(w, http.StatusBadGateway, map[string]any{
				"code":    "REVIEW_SUBMIT_FAILED",
				"message": "review request failed",
				"task":    tasksdto.TaskToResponse(t),
				"action":  tasksdto.ActionToResponse(action),
			})
			return
		}
		shared.WriteInternalError(w, err, "propose failed")
		return
	}
	var pr tasksdto.ProposeResponse
	pr.Task = tasksdto.TaskToResponse(t)
	pr.Action = tasksdto.ActionToResponse(action)
	pr.ReviewSubmit.RequestID = sub.RequestID
	pr.ReviewSubmit.Decision = sub.Decision
	pr.ReviewSubmit.Status = sub.Status
	pr.ReviewSubmit.RiskLevel = sub.RiskLevel
	pr.ReviewSubmit.DecisionReason = sub.DecisionReason
	sharedhandlers.WriteJSON(w, http.StatusOK, pr)
}

func (h *Handler) syncReview(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	t, err := h.uc.SyncTaskReview(r.Context(), id)
	if err != nil {
		if IsNotFound(err) {
			shared.WriteError(w, http.StatusNotFound, "NOT_FOUND", "task not found")
			return
		}
		shared.WriteInternalError(w, err, "sync failed")
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, tasksdto.TaskToResponse(t))
}
