package wire

import (
	"nexus-saas/cmd/config"
)

func ProvideAPIConfig(cfg config.Config) config.APIConfig               { return cfg.API }
func ProvideDBConfig(cfg config.Config) config.DBConfig                 { return cfg.DB }
func ProvideHTTPServerConfig(cfg config.Config) config.HTTPServerConfig { return cfg.HTTPServer }
func ProvideServiceConfig(cfg config.Config) config.ServiceConfig       { return cfg.Service }
