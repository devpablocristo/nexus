package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/gorm/logger"

	"gorm.io/gorm"
	"nexus-core/cmd/config"
	"nexus-core/internal/agents/coordinator"
	"nexus-core/internal/agents/mitigation"
	"nexus-core/internal/agents/recovery"
	agentruntime "nexus-core/internal/agents/runtime"
	"nexus-core/internal/agents/sentry"
	"nexus-core/internal/events"
	"nexus-core/internal/incidents"
	opsaction "nexus-core/internal/ops/actionengine"
	opseventstore "nexus-core/internal/ops/eventstore"
	opstenant "nexus-core/internal/ops/tenant"
	gormdb "nexus-core/pkg/databases/sql/gorm"
	"nexus-core/pkg/validations/jsonschema"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	db, err := gormdb.Open(gormdb.OpenOptions{DatabaseURL: cfg.DB.DatabaseURL}, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	sqlDB, _ := db.DB()
	defer func() { _ = sqlDB.Close() }()

	opsEventSvc := opseventstore.NewUsecases(
		opseventstore.NewRepository(db),
		opseventstore.NewSchemaValidator(jsonschema.NewCompilerCache(), ""),
	)
	opsEmitter := opseventstore.NewEmitter(opsEventSvc)
	opsTenantSvc := opstenant.NewUsecases(opstenant.NewRepository(db))
	opsActionSvc := opsaction.NewUsecases(opsaction.NewRepository(db))
	actionEngine := opsaction.NewEngine(
		opsActionSvc,
		opsEmitter,
		opsTenantSvc,
		opsaction.EngineConfig{},
		jsonschema.NewCompilerCache(),
	)

	legacyEventsSvc := events.NewUsecases(events.NewRepository(db))
	incidentSvc := incidents.NewUsecases(incidents.NewRepository(db), legacyEventsSvc)

	workers := []agentruntime.Worker{
		sentry.NewWorker(
			sentry.NewSentryState(db),
			incidentSvc,
			opsEmitter,
			sentry.Config{},
		),
		coordinator.NewWorker(opsEmitter),
		mitigation.NewWorker(actionEngine),
		recovery.NewWorker(opsEmitter, recovery.Config{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, w := range workers {
		r := agentruntime.NewRunner(opsEventSvc, w, 100, 700*time.Millisecond)
		go func(name string) {
			if runErr := r.Run(ctx); runErr != nil {
				fmt.Fprintf(os.Stderr, "worker %s stopped with error: %v\n", name, runErr)
			}
		}(w.ConsumerGroup())
	}

	fmt.Println("ops workers running")
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	cancel()
}
