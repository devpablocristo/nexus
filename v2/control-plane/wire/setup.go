package wire

import (
	"context"
	"net/http"
	"strings"

	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	sharedpostgres "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/postgres"
	"nexus/v2/control-plane/internal/audit"
	"nexus/v2/control-plane/internal/policies"
	"nexus/v2/control-plane/internal/resources"
)

type Config struct {
	AuditDatabaseURL        string
	ControlPlaneDatabaseURL string
}

func NewServer(cfg Config) (http.Handler, func(), error) {
	auditRepo := audit.Repository(audit.NewInMemoryRepository(nil))
	cleanups := make([]func(), 0, 2)
	if strings.TrimSpace(cfg.AuditDatabaseURL) != "" {
		postgresRepo, postgresCleanup, err := audit.NewPostgresRepository(context.Background(), cfg.AuditDatabaseURL)
		if err != nil {
			return nil, nil, err
		}
		auditRepo = postgresRepo
		cleanups = append(cleanups, postgresCleanup)
	}
	auditUC := audit.NewUsecases(auditRepo)
	auditSink := audit.NewSinkAdapter(auditUC)

	resourceRepo := resources.Repository(resources.NewInMemoryRepository(nil))
	policyRepo := policies.Repository(policies.NewInMemoryRepository(nil))
	if strings.TrimSpace(cfg.ControlPlaneDatabaseURL) != "" {
		db, err := sharedpostgres.Open(context.Background(), cfg.ControlPlaneDatabaseURL)
		if err != nil {
			cleanupCleanups(cleanups)
			return nil, nil, err
		}
		resourcePostgresRepo, err := resources.NewPostgresRepositoryWithDB(context.Background(), db)
		if err != nil {
			db.Close()
			cleanupCleanups(cleanups)
			return nil, nil, err
		}
		policyPostgresRepo, err := policies.NewPostgresRepositoryWithDB(context.Background(), db)
		if err != nil {
			db.Close()
			cleanupCleanups(cleanups)
			return nil, nil, err
		}
		resourceRepo = resourcePostgresRepo
		policyRepo = policyPostgresRepo
		cleanups = append(cleanups, db.Close)
	}
	resourceUC := resources.NewUsecases(resourceRepo)
	policyUC := policies.NewUsecases(policyRepo, policies.NewEvaluator())
	mux := http.NewServeMux()
	sharedhandlers.RegisterHealthEndpoints(mux)
	audit.NewHandler(auditUC).Register(mux)
	resources.NewHandler(resourceUC).WithAuditSink(auditSink).Register(mux)
	policies.NewHandler(policyUC).WithAuditSink(auditSink).Register(mux)
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}
	return mux, cleanup, nil
}

func cleanupCleanups(cleanups []func()) {
	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
}
