package wire

import (
	"context"
	"net/http"
	"strings"
	"time"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	"nexus/v2/data-plane/internal/action"
)

type Config struct {
	ControlPlaneURL      string
	ControlPlaneAPIKey   string
	ControlWorkersURL    string
	ControlWorkersAPIKey string
	DataPlaneDatabaseURL string
	HTTPTimeout          time.Duration
}

func NewServer(cfg Config) (http.Handler, func(), error) {
	actionRepo := action.Repository(action.NewInMemoryRepository(nil))
	cleanup := func() {}
	if strings.TrimSpace(cfg.DataPlaneDatabaseURL) != "" {
		postgresRepo, postgresCleanup, err := action.NewPostgresRepository(context.Background(), cfg.DataPlaneDatabaseURL)
		if err != nil {
			return nil, nil, err
		}
		actionRepo = postgresRepo
		cleanup = postgresCleanup
	}
	actionUsecase := action.NewUsecases(actionRepo)
	if strings.TrimSpace(cfg.ControlPlaneURL) != "" {
		controlPlaneClient := action.NewControlPlaneClient(cfg.ControlPlaneURL, cfg.HTTPTimeout).WithAPIKey(cfg.ControlPlaneAPIKey)
		actionUsecase = actionUsecase.WithResourceResolver(controlPlaneClient).WithPolicySource(controlPlaneClient)
		actionUsecase = actionUsecase.WithAuditSink(sharedaudit.NewClient(cfg.ControlPlaneURL, cfg.HTTPTimeout).WithAPIKey(cfg.ControlPlaneAPIKey))
	}
	if strings.TrimSpace(cfg.ControlWorkersURL) != "" {
		actionUsecase = actionUsecase.WithIncidentSink(action.NewControlWorkersClient(cfg.ControlWorkersURL, cfg.HTTPTimeout).WithAPIKey(cfg.ControlWorkersAPIKey))
	}

	mux := http.NewServeMux()
	sharedhandlers.RegisterHealthEndpoints(mux)
	action.NewHandler(actionUsecase).Register(mux)
	return mux, cleanup, nil
}
