package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"nexus-gateway/pkg/config/godotenv"
)

type Config struct {
	API        APIConfig
	DB         DBConfig
	HTTPServer HTTPServerConfig
	Migrations MigrationsConfig
	Service    ServiceConfig
}

func Load() (Config, error) {
	_ = godotenv.LoadIfExists()

	var cfg Config

	port, err := mustInt("NEXUS_HTTP_PORT")
	if err != nil {
		return Config{}, err
	}
	cfg.API.HTTPPort = port

	cfg.DB.DatabaseURL = os.Getenv("NEXUS_DATABASE_URL")
	if cfg.DB.DatabaseURL == "" {
		return Config{}, errors.New("NEXUS_DATABASE_URL required")
	}

	timeoutMS, err := mustIntDefault("NEXUS_HTTP_TIMEOUT_MS", 5000)
	if err != nil {
		return Config{}, err
	}
	cfg.Service.HTTPTimeoutMS = timeoutMS

	maxResp, err := mustInt64Default("NEXUS_HTTP_MAX_RESPONSE_BYTES", 1048576)
	if err != nil {
		return Config{}, err
	}
	cfg.Service.HTTPMaxResponseBytes = maxResp

	maxBody, err := mustInt64Default("NEXUS_MAX_BODY_BYTES", 262144)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPServer.MaxBodyBytes = maxBody

	cfg.Service.LogLevel = mustStrDefault("NEXUS_LOG_LEVEL", "info")

	cfg.Service.SwaggerCDN = mustBoolDefault("NEXUS_SWAGGER_CDN", true)

	rl, err := mustIntDefault("NEXUS_RATE_LIMIT_DEFAULT_PER_MINUTE", 60)
	if err != nil {
		return Config{}, err
	}
	cfg.Service.RateLimitDefaultPerMin = rl

	cfg.Migrations.Dir = mustStrDefault("NEXUS_MIGRATIONS_DIR", "./migrations")

	return cfg, nil
}

func mustStrDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func mustInt(key string) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return 0, fmt.Errorf("%s required", key)
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s invalid int: %w", key, err)
	}
	return i, nil
}

func mustIntDefault(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s invalid int: %w", key, err)
	}
	return i, nil
}

func mustInt64Default(key string, def int64) (int64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s invalid int64: %w", key, err)
	}
	return i, nil
}

func mustBoolDefault(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
