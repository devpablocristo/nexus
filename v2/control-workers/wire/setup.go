package wire

import (
	"net/http"

	"nexus/v2/control-workers/internal/alerts"
	"nexus/v2/control-workers/internal/incidents"
)

func NewServer() http.Handler {
	alertRepo := alerts.NewInMemoryRepository(nil)
	alertUC := alerts.NewUsecases(alertRepo)
	incidentRepo := incidents.NewInMemoryRepository(nil)
	incidentUC := incidents.NewUsecases(incidentRepo).WithAlertSink(alertUC)
	mux := http.NewServeMux()
	alerts.NewHandler(alertUC).Register(mux)
	incidents.NewHandler(incidentUC).Register(mux)
	return mux
}
