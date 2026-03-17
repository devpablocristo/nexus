package audit

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
	mux.HandleFunc("GET /v1/requests/{id}/replay", h.replay)
}

func (h *Handler) replay(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		sharedhandlers.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		sharedhandlers.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	out, err := h.uc.Replay(r.Context(), id)
	if err != nil {
		sharedhandlers.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, out)
}
