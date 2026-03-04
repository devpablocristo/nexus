package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"nexus-control-operators/internal/agents/coordinator"
	"nexus-control-operators/internal/agents/mitigation"
	"nexus-control-operators/internal/agents/recovery"
	agentruntime "nexus-control-operators/internal/agents/runtime"
	"nexus-control-operators/internal/agents/sentry"
	"nexus-control-operators/internal/coreproxy"
	opseventstore "nexus-control-operators/internal/ops/eventstore"
	"nexus/pkg/validations/jsonschema"
)

type WorkerConfig struct {
	CoreURL        string
	InternalKey    string
	DefaultOrgID   uuid.UUID
	BatchSize      int
	PollIntervalMS int
	HealthPort     string
	LogLevel       string
}

func loadWorkerConfig() (WorkerConfig, error) {
	cfg := WorkerConfig{
		CoreURL:        envOrDefault("NEXUS_CORE_URL", "http://nexus-core:8080"),
		InternalKey:    firstNonEmpty(os.Getenv("OPERATOR_INTERNAL_KEY"), os.Getenv("NEXUS_AI_OPERATORS_INTERNAL_KEY")),
		BatchSize:      envIntOrDefault("OPERATOR_BATCH_SIZE", 100),
		PollIntervalMS: envIntOrDefault("OPERATOR_POLL_INTERVAL_MS", 700),
		HealthPort:     envOrDefault("OPERATOR_HEALTH_PORT", "8090"),
		LogLevel:       envOrDefault("NEXUS_LOG_LEVEL", "info"),
	}
	if cfg.InternalKey == "" {
		return cfg, &configError{"OPERATOR_INTERNAL_KEY or NEXUS_AI_OPERATORS_INTERNAL_KEY required"}
	}
	if rawOrg := strings.TrimSpace(os.Getenv("NEXUS_DEFAULT_ORG_ID")); rawOrg != "" {
		if parsed, err := uuid.Parse(rawOrg); err == nil {
			cfg.DefaultOrgID = parsed
		}
	}
	return cfg, nil
}

func main() {
	cfg, err := loadWorkerConfig()
	if err != nil {
		bootLog := zerolog.New(os.Stderr).With().Timestamp().Logger()
		bootLog.Fatal().Err(err).Msg("config load failed")
	}

	level, _ := zerolog.ParseLevel(cfg.LogLevel)
	if level == zerolog.NoLevel {
		level = zerolog.InfoLevel
	}
	log := zerolog.New(os.Stderr).With().Timestamp().Logger().Level(level)

	coreClient := coreproxy.NewClient(cfg.CoreURL, cfg.InternalKey, 3*time.Second, log)
	eventRepo := coreproxy.NewEventstoreRepository(coreClient, cfg.DefaultOrgID)

	opsEventSvc := opseventstore.NewUsecases(
		eventRepo,
		opseventstore.NewSchemaValidator(jsonschema.NewCompilerCache(), ""),
	)
	opsEmitter := opseventstore.NewEmitter(opsEventSvc)
	actionEngine := coreproxy.NewCoreActionEngine(coreClient)
	incidentSvc := coreproxy.NewIncidentsClient(coreClient)

	workers := []agentruntime.Worker{
		sentry.NewWorker(sentry.NewInMemoryState(), incidentSvc, opsEmitter, sentry.Config{}, log),
		coordinator.NewWorker(opsEmitter, log),
		mitigation.NewWorker(actionEngine),
		recovery.NewWorker(opsEmitter, recovery.Config{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pollInterval := time.Duration(cfg.PollIntervalMS) * time.Millisecond

	var wg sync.WaitGroup
	for _, w := range workers {
		wg.Add(1)
		r := agentruntime.NewRunner(opsEventSvc, w, cfg.BatchSize, pollInterval, log)
		go func(name string) {
			defer wg.Done()
			if runErr := r.Run(ctx); runErr != nil {
				log.Error().Err(runErr).Str("worker", name).Msg("worker stopped")
			}
		}(w.ConsumerGroup())
	}

	go startHealthServer(cfg.HealthPort, log)

	log.Info().Int("workers", len(workers)).Str("core_url", cfg.CoreURL).Msg("ops workers running")
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Info().Msg("shutdown signal received, draining workers")
	cancel()
	wg.Wait()
	log.Info().Msg("all workers stopped, exiting")
}

func startHealthServer(port string, log zerolog.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	addr := ":" + port
	log.Info().Str("addr", addr).Msg("health server listening")
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error().Err(err).Msg("health server failed")
	}
}

func envOrDefault(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func envIntOrDefault(key string, def int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return def
	}
	return v
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

type configError struct{ msg string }

func (e *configError) Error() string { return e.msg }
