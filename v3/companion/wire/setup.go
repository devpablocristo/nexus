package wire

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	sharedapikey "github.com/devpablocristo/core/backend/go/apikey"
	sharedhandlers "github.com/devpablocristo/core/backend/go/httpjson"
	sharedpostgres "github.com/devpablocristo/core/databases/postgres/go"
	"github.com/devpablocristo/nexus/v3/companion/internal/reviewclient"
	"github.com/devpablocristo/nexus/v3/companion/internal/tasks"
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

// Config arranque del servicio Companion.
type Config struct {
	DatabaseURL    string
	APIKeys        string
	ReviewBaseURL  string
	ReviewAPIKey   string
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

	mux := http.NewServeMux()
	sharedhandlers.RegisterHealthEndpoints(mux, func(c context.Context) error {
		return db.Ping(c)
	})
	h.Register(mux)

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

	return authn.Middleware(mux), cleanup, nil
}
