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
		Port:              mustIntDefault("SIM_ENGINE_PORT", 8087),
		DatabaseURL:       os.Getenv("SIM_ENGINE_DATABASE_URL"),
		InternalKey:       os.Getenv("NEXUS_SIM_ENGINE_INTERNAL_KEY"),
		DefaultAgentCount: mustIntDefault("SIM_ENGINE_DEFAULT_AGENT_COUNT", 50),
		SnapshotEvery:     int64(mustIntDefault("SIM_ENGINE_SNAPSHOT_EVERY", 1)),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("SIM_ENGINE_DATABASE_URL required")
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
