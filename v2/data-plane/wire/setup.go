package wire

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	sharedobservability "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/observability"
	sharedpostgres "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/postgres"
	"nexus/v2/data-plane/internal/action"
)

type Config struct {
	ControlPlaneURL         string
	ControlPlaneAPIKey      string
	ControlWorkersURL       string
	ControlWorkersAPIKey    string
	DataPlaneDatabaseURL    string
	DataPlanePostgresConfig sharedpostgres.Config
	HTTPTimeout             time.Duration
	Metrics                 *sharedobservability.Metrics
}

func NewServer(cfg Config) (http.Handler, func(), error) {
	actionRepo := action.Repository(action.NewInMemoryRepository(nil))
	idempotencyStore := action.IdempotencyStore(action.NewInMemoryIdempotencyStore())
	riskStore := action.RiskBaselineStore(action.NewInMemoryRiskBaselineStore())
	cleanup := func() {}
	var readinessChecks []sharedhandlers.ReadinessCheck
	var riskProvider *action.HistoricalRiskContextProvider
	if strings.TrimSpace(cfg.DataPlaneDatabaseURL) != "" {
		db, err := sharedpostgres.OpenWithConfig(context.Background(), cfg.DataPlaneDatabaseURL, cfg.DataPlanePostgresConfig)
		if err != nil {
			return nil, nil, err
		}
		postgresRepo, err := action.NewPostgresRepositoryWithDB(context.Background(), db)
		if err != nil {
			db.Close()
			return nil, nil, err
		}
		actionRepo = postgresRepo
		idempotencyStore = action.NewPostgresIdempotencyStore(db.Pool())
		riskStore = action.NewPostgresRiskBaselineStore(db)
		cleanup = db.Close
		readinessChecks = append(readinessChecks, db.Ping)
	}
	actionUsecase := action.NewUsecases(actionRepo)
	riskProvider = action.NewHistoricalRiskContextProvider(actionRepo, riskStore)
	actionUsecase = actionUsecase.WithRiskContextProvider(riskProvider)
	if cfg.Metrics != nil {
		actionUsecase = actionUsecase.WithMetrics(cfg.Metrics)
	}
	if strings.TrimSpace(cfg.ControlPlaneURL) != "" {
		controlPlaneClient := action.NewControlPlaneClient(cfg.ControlPlaneURL, cfg.HTTPTimeout).WithAPIKey(cfg.ControlPlaneAPIKey)
		cacheConfig := action.DefaultCacheConfig()
		logger := slog.Default()
		cachedResolver := action.NewCachingResourceResolver(controlPlaneClient, cacheConfig, logger)
		cachedPolicySource := action.NewCachingPolicySource(controlPlaneClient, cacheConfig, logger)
		actionUsecase = actionUsecase.WithResourceResolver(cachedResolver).WithPolicySource(cachedPolicySource)
		actionUsecase = actionUsecase.WithAuditSink(sharedaudit.NewClient(cfg.ControlPlaneURL, cfg.HTTPTimeout).WithAPIKey(cfg.ControlPlaneAPIKey))
	}
	if strings.TrimSpace(cfg.ControlWorkersURL) != "" {
		controlWorkersClient := action.NewControlWorkersClient(cfg.ControlWorkersURL, cfg.HTTPTimeout).WithAPIKey(cfg.ControlWorkersAPIKey)
		actionUsecase = actionUsecase.WithIncidentSink(controlWorkersClient)
		riskProvider = riskProvider.WithIncidentReader(controlWorkersClient)
	}
	riskProvider.Start(context.Background(), time.Hour, slog.Default())

	mux := http.NewServeMux()
	sharedhandlers.RegisterHealthEndpoints(mux, sharedhandlers.ComposeReadinessChecks(readinessChecks...))
	action.NewHandler(actionUsecase).WithIdempotency(idempotencyStore).Register(mux)
	return mux, cleanup, nil
}
