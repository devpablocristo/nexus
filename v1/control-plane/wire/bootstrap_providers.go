package wire

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"control-plane/cmd/config"
	"control-plane/internal/admin"
	"control-plane/internal/alerts"
	"control-plane/internal/billing"
	gormdb "nexus/pkg/databases/sql/gorm"
)

type App struct {
	Server            *http.Server
	startBackgroundFn func(ctx context.Context)
}

func NewLogger(cfg config.ServiceConfig) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	return zerolog.New(os.Stdout).Level(lvl).With().Timestamp().Logger()
}

func NewGormConfig(_ zerolog.Logger) *gorm.Config {
	return &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Warn)}
}

func NewDB(cfg config.DBConfig, gormCfg *gorm.Config) (*gorm.DB, func(), error) {
	db, err := gormdb.Open(gormdb.OpenOptions{DatabaseURL: cfg.DatabaseURL}, gormCfg)
	if err != nil {
		return nil, nil, err
	}
	if err := ensureSaaSSchema(db); err != nil {
		return nil, nil, err
	}
	closeFn := func() {
		sqlDB, err := db.DB()
		if err != nil {
			return
		}
		_ = sqlDB.Close()
	}
	return db, closeFn, nil
}

func NewHTTPServer(cfg config.APIConfig, router *gin.Engine) *http.Server {
	return &http.Server{
		Addr:              ":" + itoa(cfg.HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func NewApp(
	server *http.Server,
	alertsUC *alerts.Usecases,
	billingRepo *billing.Repository,
	adminUC *admin.Usecases,
	cfg config.ServiceConfig,
	logger zerolog.Logger,
) *App {
	interval := cfg.AlertEvalInterval
	startFn := func(ctx context.Context) {
		if alertsUC != nil && interval > 0 {
			workerLog := logger.With().Str("component", "alerts-evaluator").Logger()
			ticker := time.NewTicker(interval)
			go func() {
				workerLog.Info().Dur("interval", interval).Msg("alert evaluator started")
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						fired, err := alertsUC.EvaluateAll(ctx)
						if err != nil {
							workerLog.Error().Err(err).Msg("alert evaluation failed")
							continue
						}
						workerLog.Info().Int("fired", fired).Msg("alert evaluation completed")
					case <-ctx.Done():
						workerLog.Info().Msg("alert evaluator stopped")
						return
					}
				}
			}()
		}

		if billingRepo != nil && adminUC != nil {
			dunningLog := logger.With().Str("component", "billing-dunning").Logger()
			worker := billing.NewDunningWorker(billingRepo, adminUC, dunningLog)
			ticker := time.NewTicker(24 * time.Hour)
			go func() {
				dunningLog.Info().Dur("interval", 24*time.Hour).Msg("dunning worker started")
				defer ticker.Stop()
				worker.RunOnce(ctx)
				for {
					select {
					case <-ticker.C:
						worker.RunOnce(ctx)
					case <-ctx.Done():
						dunningLog.Info().Msg("dunning worker stopped")
						return
					}
				}
			}()
		}
	}
	return &App{
		Server:            server,
		startBackgroundFn: startFn,
	}
}

func (a *App) StartBackgroundWorkers(ctx context.Context) {
	if a == nil || a.startBackgroundFn == nil {
		return
	}
	a.startBackgroundFn(ctx)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var b [32]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
