package wire

import (
	"context"
	"net/http"
	"strings"

	sharedapikey "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/apikey"
	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	sharedpostgres "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/postgres"
	"nexus/v2/control-plane/internal/audit"
	"nexus/v2/control-plane/internal/policies"
	"nexus/v2/control-plane/internal/resources"
)

type Config struct {
	AuditDatabaseURL           string
	ControlPlaneDatabaseURL    string
	AuditPostgresConfig        sharedpostgres.Config
	ControlPlanePostgresConfig sharedpostgres.Config
	NexusAPIKeys               string
	SaaS                       SaaSConfig
}

func NewServer(cfg Config) (http.Handler, func(), error) {
	auditRepo := audit.Repository(audit.NewInMemoryRepository(nil))
	cleanups := make([]func(), 0, 2)
	readinessChecks := make([]sharedhandlers.ReadinessCheck, 0, 2)
	if strings.TrimSpace(cfg.AuditDatabaseURL) != "" {
		db, err := sharedpostgres.OpenWithConfig(context.Background(), cfg.AuditDatabaseURL, cfg.AuditPostgresConfig)
		if err != nil {
			return nil, nil, err
		}
		postgresRepo, err := audit.NewPostgresRepositoryWithDB(context.Background(), db)
		if err != nil {
			db.Close()
			return nil, nil, err
		}
		auditRepo = postgresRepo
		cleanups = append(cleanups, db.Close)
		readinessChecks = append(readinessChecks, db.Ping)
	}
	auditUC := audit.NewUsecases(auditRepo)
	auditSink := audit.NewSinkAdapter(auditUC)

	resourceRepo := resources.Repository(resources.NewInMemoryRepository(nil))
	policyRepo := policies.Repository(policies.NewInMemoryRepository(nil))
	if strings.TrimSpace(cfg.ControlPlaneDatabaseURL) != "" {
		db, err := sharedpostgres.OpenWithConfig(context.Background(), cfg.ControlPlaneDatabaseURL, cfg.ControlPlanePostgresConfig)
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
		readinessChecks = append(readinessChecks, db.Ping)
	}
	resourceUC := resources.NewUsecases(resourceRepo)
	policyUC := policies.NewUsecases(policyRepo, policies.NewEvaluator())
	if err := policyUC.EnsureCanaryTrapPolicy(context.Background()); err != nil {
		cleanupCleanups(cleanups)
		return nil, nil, err
	}
	// SaaS modules (billing, auth, tenancy)
	saasSvc, err := SetupSaaS(cfg.SaaS)
	if err != nil {
		cleanupCleanups(cleanups)
		return nil, nil, err
	}
	if saasSvc != nil {
		cleanups = append(cleanups, saasSvc.Cleanup)
	}

	mux := http.NewServeMux()
	sharedhandlers.RegisterHealthEndpoints(mux, sharedhandlers.ComposeReadinessChecks(readinessChecks...))
	audit.NewHandler(auditUC).Register(mux)
	resources.NewHandler(resourceUC).WithAuditSink(auditSink).Register(mux)
	policies.NewHandler(policyUC).WithAuditSink(auditSink).Register(mux)
	RegisterSaaSRoutes(mux, saasSvc)

	authn, err := sharedapikey.NewAuthenticator(cfg.NexusAPIKeys)
	if err != nil {
		cleanupCleanups(cleanups)
		return nil, nil, err
	}

	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}
	return WrapAuth(mux, authn, saasSvc), cleanup, nil
}

func cleanupCleanups(cleanups []func()) {
	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
}
