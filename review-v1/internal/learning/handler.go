package learning

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
	mux.HandleFunc("GET /v1/learning/proposals", h.listProposals)
	mux.HandleFunc("GET /v1/learning/proposals/{id}", h.getProposal)
	mux.HandleFunc("POST /v1/learning/proposals/{id}/accept", h.accept)
	mux.HandleFunc("POST /v1/learning/proposals/{id}/dismiss", h.dismiss)
}

func (h *Handler) listProposals(w http.ResponseWriter, r *http.Request) {
	list, err := h.uc.ListPendingProposals(r.Context(), 50)
	if err != nil {
		sharedhandlers.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (h *Handler) getProposal(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		sharedhandlers.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	p, err := h.uc.GetProposalByID(r.Context(), id)
	if err != nil {
		if err == ErrNotFound {
			sharedhandlers.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		sharedhandlers.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, p)
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		sharedhandlers.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var body struct {
		DecidedBy string `json:"decided_by"`
	}
	_ = sharedhandlers.DecodeJSON(r, &body)
	policyID, err := h.uc.AcceptProposal(r.Context(), id, body.DecidedBy)
	if err != nil {
		if err == ErrNotPending {
			sharedhandlers.WriteJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		sharedhandlers.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	resp := map[string]any{"status": "accepted"}
	if policyID != nil {
		resp["policy_id"] = policyID.String()
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) dismiss(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		sharedhandlers.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var body struct {
		DecidedBy string `json:"decided_by"`
	}
	_ = sharedhandlers.DecodeJSON(r, &body)
	if err := h.uc.DismissProposal(r.Context(), id, body.DecidedBy); err != nil {
		if err == ErrNotPending {
			sharedhandlers.WriteJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		sharedhandlers.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
}
