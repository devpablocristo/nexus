package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"nexus-control-operators/cmd/config"
	"nexus-control-operators/internal/agents/coordinator"
	"nexus-control-operators/internal/agents/mitigation"
	"nexus-control-operators/internal/agents/recovery"
	agentruntime "nexus-control-operators/internal/agents/runtime"
	"nexus-control-operators/internal/agents/sentry"
	"nexus-control-operators/internal/coreproxy"
	opseventstore "nexus-control-operators/internal/ops/eventstore"
	"nexus/pkg/validations/jsonschema"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	_ = cfg

	coreURL := strings.TrimSpace(os.Getenv("NEXUS_CORE_URL"))
	if coreURL == "" {
		coreURL = "http://nexus-core:8080"
	}
	internalKey := strings.TrimSpace(os.Getenv("OPERATOR_INTERNAL_KEY"))
	if internalKey == "" {
		internalKey = strings.TrimSpace(os.Getenv("NEXUS_AI_OPERATORS_INTERNAL_KEY"))
	}
	if internalKey == "" {
		fmt.Fprintln(os.Stderr, "OPERATOR_INTERNAL_KEY or NEXUS_AI_OPERATORS_INTERNAL_KEY required")
		os.Exit(1)
	}
	defaultOrgID := uuid.Nil
	if rawOrg := strings.TrimSpace(os.Getenv("NEXUS_DEFAULT_ORG_ID")); rawOrg != "" {
		if parsed, parseErr := uuid.Parse(rawOrg); parseErr == nil {
			defaultOrgID = parsed
		}
	}
	coreClient := coreproxy.NewClient(coreURL, internalKey, 3*time.Second)
	eventRepo := coreproxy.NewEventstoreRepository(coreClient, defaultOrgID)

	opsEventSvc := opseventstore.NewUsecases(
		eventRepo,
		opseventstore.NewSchemaValidator(jsonschema.NewCompilerCache(), ""),
	)
	opsEmitter := opseventstore.NewEmitter(opsEventSvc)
	actionEngine := coreproxy.NewCoreActionEngine(coreClient)
	incidentSvc := coreproxy.NewIncidentsClient(coreClient)

	workers := []agentruntime.Worker{
		sentry.NewWorker(
			sentry.NewInMemoryState(),
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
