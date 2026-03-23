package wire

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	sharedapikey "github.com/devpablocristo/core/backend/go/apikey"
	sharedhandlers "github.com/devpablocristo/core/backend/go/httpjson"
	sharedpostgres "github.com/devpablocristo/core/databases/postgres/go"
	"github.com/devpablocristo/nexus/v3/companion/internal/connectors"
	"github.com/devpablocristo/nexus/v3/companion/internal/connectors/registry"
	"github.com/devpablocristo/nexus/v3/companion/internal/memory"
	"github.com/devpablocristo/core/governance/go/reviewclient"
	"github.com/devpablocristo/nexus/v3/companion/internal/tasks"
	"github.com/devpablocristo/nexus/v3/companion/internal/watchers"
	"github.com/devpablocristo/nexus/v3/companion/internal/watchers/pymesclient"
)

func reviewSyncInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("COMPANION_REVIEW_SYNC_INTERVAL_SEC"))
	if raw == "" {
		return 30 * time.Second
	}
	sec, err := strconv.Atoi(raw)
	if err != nil || sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}

func watcherInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("COMPANION_WATCHER_INTERVAL_SEC"))
	if raw == "" {
		return 0
	}
	sec, err := strconv.Atoi(raw)
	if err != nil || sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}

// Config arranque del servicio Companion.
type Config struct {
	DatabaseURL    string
	APIKeys        string
	ReviewBaseURL  string
	ReviewAPIKey   string
	PymesBaseURL   string
	PymesAPIKey    string
	MigrationFiles fs.FS
}

// NewServer abre DB, migra, monta mux y auth.
func NewServer(cfg Config) (http.Handler, func(), error) {
	ctx := context.Background()

	db, err := sharedpostgres.OpenWithConfig(ctx, cfg.DatabaseURL, sharedpostgres.DefaultConfig("nexus-companion"))
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	if err := sharedpostgres.MigrateUp(ctx, db, "nexus-companion", cfg.MigrationFiles, "."); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	repo := tasks.NewPostgresRepository(db)
	rc := reviewclient.NewClient(cfg.ReviewBaseURL, cfg.ReviewAPIKey)
	uc := tasks.NewUsecases(repo, rc)
	h := tasks.NewHandler(uc)

	// Watchers module
	watcherRepo := watchers.NewPostgresRepository(db)
	pymesClient := pymesclient.NewClient(cfg.PymesBaseURL, cfg.PymesAPIKey)
	watcherUC := watchers.NewUsecases(watcherRepo, pymesClient, rc)
	watcherHandler := watchers.NewHandler(watcherUC)

	// Memory module
	memRepo := memory.NewPostgresRepository(db)
	memUC := memory.NewUsecases(memRepo)
	memHandler := memory.NewHandler(memUC)

	// Connectors module
	connReg := registry.NewRegistry()
	connReg.Register(registry.NewMockConnector())
	if cfg.PymesBaseURL != "" {
		connReg.Register(registry.NewPymesConnector(pymesClient))
	}
	connRepo := connectors.NewPostgresRepository(db)
	reviewChecker := connectors.NewReviewCheckerAdapter(func(c context.Context, id uuid.UUID) (string, int, error) {
		summary, status, err := rc.GetRequest(c, id.String())
		if err != nil {
			return "", status, err
		}
		return summary.Status, status, nil
	})
	connUC := connectors.NewUsecases(connRepo, connReg, reviewChecker)
	connHandler := connectors.NewHandler(connUC)

	mux := http.NewServeMux()
	sharedhandlers.RegisterHealthEndpoints(mux, func(c context.Context) error {
		return db.Ping(c)
	})
	h.Register(mux)
	watcherHandler.Register(mux)
	memHandler.Register(mux)
	connHandler.Register(mux)

	// Seed conectores por defecto
	if err := connUC.SeedDefaultConnectors(ctx); err != nil {
		slog.Error("seed default connectors", "error", err)
	}

	authn, err := sharedapikey.NewAuthenticator(cfg.APIKeys)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("create authenticator: %w", err)
	}

	cleanup := func() {
		db.Close()
	}
	if d := reviewSyncInterval(); d > 0 {
		syncCtx, syncCancel := context.WithCancel(context.Background())
		go uc.RunReviewSyncLoop(syncCtx, d, 50)
		prev := cleanup
		cleanup = func() {
			syncCancel()
			prev()
		}
	}
	if d := watcherInterval(); d > 0 {
		watcherCtx, watcherCancel := context.WithCancel(context.Background())
		go watcherUC.RunWatcherLoop(watcherCtx, d, 50)
		prev := cleanup
		cleanup = func() {
			watcherCancel()
			prev()
		}
	}

	// Memory purge loop: limpia entradas expiradas cada hora
	{
		purgeCtx, purgeCancel := context.WithCancel(context.Background())
		go memUC.RunPurgeLoop(purgeCtx, 1*time.Hour)
		prev := cleanup
		cleanup = func() {
			purgeCancel()
			prev()
		}
	}

	return authn.Middleware(mux), cleanup, nil
}
