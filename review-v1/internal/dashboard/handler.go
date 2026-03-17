package dashboard

import (
	"net/http"

	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/metrics/summary", h.summary)
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}
	// Stub: real implementation would query requests/approvals
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]any{
		"period":           period,
		"total_requests":   0,
		"allowed":          0,
		"denied":           0,
		"pending_approval": 0,
		"approved":         0,
		"rejected":         0,
	})
}
