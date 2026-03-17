package approvals

import (
	"net/http"

	"github.com/google/uuid"
	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
)

type Handler struct {
	uc *Usecases
}

func NewHandler(uc *Usecases) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/approvals/pending", h.listPending)
	mux.HandleFunc("POST /v1/approvals/{id}/approve", h.approve)
	mux.HandleFunc("POST /v1/approvals/{id}/reject", h.reject)
}

func (h *Handler) listPending(w http.ResponseWriter, r *http.Request) {
	list, err := h.uc.ListPending(r.Context(), 50)
	if err != nil {
		sharedhandlers.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		sharedhandlers.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var body struct {
		DecidedBy string `json:"decided_by"`
		Note      string `json:"note"`
	}
	_ = sharedhandlers.DecodeJSON(r, &body)
	if err := h.uc.Approve(r.Context(), id, body.DecidedBy, body.Note); err != nil {
		if err == ErrNotPending {
			sharedhandlers.WriteJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		sharedhandlers.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		sharedhandlers.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var body struct {
		DecidedBy string `json:"decided_by"`
		Note      string `json:"note"`
	}
	_ = sharedhandlers.DecodeJSON(r, &body)
	if err := h.uc.Reject(r.Context(), id, body.DecidedBy, body.Note); err != nil {
		if err == ErrNotPending {
			sharedhandlers.WriteJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		sharedhandlers.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}
