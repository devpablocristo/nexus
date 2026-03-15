package wire

import (
	"context"
	"net/http"
	"strings"
	"time"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	sharedpostgres "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/postgres"
	"nexus/v2/control-workers/internal/alerts"
	"nexus/v2/control-workers/internal/incidents"
)

type Config struct {
	ControlPlaneURL           string
	ControlPlaneAPIKey        string
	ControlWorkersDatabaseURL string
	HTTPTimeout               time.Duration
}

func NewServer(cfg Config) (http.Handler, func(), error) {
	alertRepo := alerts.Repository(alerts.NewInMemoryRepository(nil))
	incidentRepo := incidents.Repository(incidents.NewInMemoryRepository(nil))
	cleanups := make([]func(), 0, 1)
	if strings.TrimSpace(cfg.ControlWorkersDatabaseURL) != "" {
		db, err := sharedpostgres.Open(context.Background(), cfg.ControlWorkersDatabaseURL)
		if err != nil {
			return nil, nil, err
		}
		alertPostgresRepo, err := alerts.NewPostgresRepositoryWithDB(context.Background(), db)
		if err != nil {
			db.Close()
			return nil, nil, err
		}
		incidentPostgresRepo, err := incidents.NewPostgresRepositoryWithDB(context.Background(), db)
		if err != nil {
			db.Close()
			return nil, nil, err
		}
		alertRepo = alertPostgresRepo
		incidentRepo = incidentPostgresRepo
		cleanups = append(cleanups, db.Close)
	}
	alertUC := alerts.NewUsecases(alertRepo)
	incidentUC := incidents.NewUsecases(incidentRepo).WithAlertSink(alertUC)
	if strings.TrimSpace(cfg.ControlPlaneURL) != "" {
		auditClient := sharedaudit.NewClient(cfg.ControlPlaneURL, cfg.HTTPTimeout).WithAPIKey(cfg.ControlPlaneAPIKey)
		alertUC = alertUC.WithAuditSink(auditClient)
		incidentUC = incidentUC.WithAuditSink(auditClient)
	}
	mux := http.NewServeMux()
	sharedhandlers.RegisterHealthEndpoints(mux)
	alerts.NewHandler(alertUC).Register(mux)
	incidents.NewHandler(incidentUC).Register(mux)
	return mux, func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}, nil
}
