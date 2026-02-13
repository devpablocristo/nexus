package wire

import (
	"nexus-gateway/cmd/config"
	gwuc "nexus-gateway/internal/gateway/usecases"
)

func ProvideAPIConfig(cfg config.Config) config.APIConfig               { return cfg.API }
func ProvideDBConfig(cfg config.Config) config.DBConfig                 { return cfg.DB }
func ProvideHTTPServerConfig(cfg config.Config) config.HTTPServerConfig { return cfg.HTTPServer }
func ProvideServiceConfig(cfg config.Config) config.ServiceConfig       { return cfg.Service }
func ProvideMigrationsConfig(cfg config.Config) config.MigrationsConfig { return cfg.Migrations }

func ProvideGatewayConfig(cfg config.ServiceConfig) gwuc.Config {
	return gwuc.Config{
		DefaultRateLimitPerMinute: cfg.RateLimitDefaultPerMin,
		MaxBytesInputDefault:      262144,
		MaxBytesContextDefault:    65536,
	}
}
