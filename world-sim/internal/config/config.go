package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	Port              int
	DatabaseURL       string
	InternalKey       string
	DefaultAgentCount int
	SnapshotEvery     int64
}

func Load() (Config, error) {
	cfg := Config{
		Port:              mustIntDefault("WORLDSIM_PORT", 8087),
		DatabaseURL:       os.Getenv("WORLDSIM_DATABASE_URL"),
		InternalKey:       os.Getenv("NEXUS_WORLDSIM_INTERNAL_KEY"),
		DefaultAgentCount: mustIntDefault("WORLDSIM_DEFAULT_AGENT_COUNT", 50),
		SnapshotEvery:     int64(mustIntDefault("WORLDSIM_SNAPSHOT_EVERY", 1)),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("WORLDSIM_DATABASE_URL required")
	}
	if cfg.DefaultAgentCount <= 0 {
		cfg.DefaultAgentCount = 50
	}
	if cfg.SnapshotEvery <= 0 {
		cfg.SnapshotEvery = 1
	}
	return cfg, nil
}

func mustIntDefault(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}
