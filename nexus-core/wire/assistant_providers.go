package wire

import (
	"os"
	"time"

	"github.com/google/wire"

	"nexus-core/internal/assistant"
)

func ProvideAssistantConfig() assistant.Config {
	baseURL := os.Getenv("NEXUS_OPERATOR_URL")
	if baseURL == "" {
		baseURL = "http://nexus-external-operators:8000"
	}
	apiKey := os.Getenv("NEXUS_OPERATOR_INTERNAL_KEY")
	return assistant.Config{
		OperatorBaseURL: baseURL,
		OperatorAPIKey:  apiKey,
		Timeout:         6 * time.Second,
	}
}

var AssistantSet = wire.NewSet(
	ProvideAssistantConfig,
	assistant.NewUsecases,
	assistant.NewHandler,
)
