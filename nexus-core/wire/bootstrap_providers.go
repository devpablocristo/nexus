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

	"nexus-core/cmd/config"
	"nexus-core/internal/dlp"
	gormdb "nexus/pkg/databases/sql/gorm"
	"nexus/pkg/utils"
	"nexus/pkg/validations/jsonschema"
)

type ActionTTLEngine interface {
	ExpireDue(ctx context.Context, now time.Time, batch int) (int, error)
}

type App struct {
	Router         *gin.Engine
	Server         *http.Server
	ActionTTLEngine ActionTTLEngine
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
	closeFn := func() {
		sqlDB, err := db.DB()
		if err != nil {
			return
		}
		_ = sqlDB.Close()
	}
	return db, closeFn, nil
}

func NewSchemaCache() *jsonschema.CompilerCache {
	return jsonschema.NewCompilerCache()
}

func NewDLPDetector() *dlp.Detector {
	return dlp.NewDetector()
}

func NewMasterCrypto(cfg config.ServiceConfig) (*utils.AESGCM, error) {
	return utils.NewAESGCM(cfg.MasterKey)
}

func NewHTTPServer(cfg config.APIConfig, router *gin.Engine) *http.Server {
	return &http.Server{
		Addr:              ":" + itoa(cfg.HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func NewApp(router *gin.Engine, server *http.Server) *App {
	return &App{
		Router: router,
		Server: server,
	}
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
