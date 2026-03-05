package config

import (
	"fmt"
	"os"
	"strconv"

	"nexus/pkg/config/godotenv"
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

	if err := loadDB(&cfg); err != nil {
		return Config{}, err
	}
	if err := loadHTTP(&cfg); err != nil {
		return Config{}, err
	}

	cfg.Service.LogLevel = mustStrDefault("NEXUS_LOG_LEVEL", "info")
	cfg.Service.SwaggerCDN = mustBoolDefault("NEXUS_SWAGGER_CDN", true)

	if err := loadAuth(&cfg); err != nil {
		return Config{}, err
	}
	if err := loadRateLimit(&cfg); err != nil {
		return Config{}, err
	}
	if err := loadOTel(&cfg); err != nil {
		return Config{}, err
	}
	if err := loadGateway(&cfg); err != nil {
		return Config{}, err
	}
	if err := loadOIDC(&cfg); err != nil {
		return Config{}, err
	}
	if err := loadBilling(&cfg); err != nil {
		return Config{}, err
	}

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
