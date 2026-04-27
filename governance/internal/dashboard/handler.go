package dashboard

import (
	"context"
	"net/http"

	"github.com/devpablocristo/core/http/go/httpjson"
	dashboarddto "github.com/devpablocristo/nexus/governance/internal/dashboard/handler/dto"
	requestdomain "github.com/devpablocristo/nexus/governance/internal/requests/usecases/domain"
)

const maxListLimit = 1000

type requestLister interface {
	List(ctx context.Context, status, actionType string, limit int) ([]requestdomain.Request, error)
}

type Handler struct {
	requests requestLister
}

func NewHandler(requests requestLister) *Handler {
	return &Handler{requests: requests}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/metrics/summary", h.summary)
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}
	all, err := h.requests.List(r.Context(), "", "", maxListLimit)
	if err != nil {
		httpjson.WriteFlatInternalError(w, err, "dashboard query failed")
		return
	}

	var allowed, denied, pendingApproval, approved, rejected int
	for _, req := range all {
		switch req.Status {
		case requestdomain.StatusAllowed:
			allowed++
		case requestdomain.StatusDenied:
			denied++
		case requestdomain.StatusPendingApproval:
			pendingApproval++
		case requestdomain.StatusApproved, requestdomain.StatusExecuted:
			approved++
		case requestdomain.StatusRejected:
			rejected++
		}
	}

	httpjson.WriteJSON(w, http.StatusOK, dashboarddto.SummaryResponse{
		Period:          period,
		TotalRequests:   len(all),
		Allowed:         allowed,
		Denied:          denied,
		PendingApproval: pendingApproval,
		Approved:        approved,
		Rejected:        rejected,
	})
}
