package llm

import (
	"os"
	"time"
)

type Config struct {
	Provider      string
	Model         string
	OllamaBaseURL string
	CloudBaseURL  string
	CloudAPIKey   string
	SchemaDir     string
	Timeout       time.Duration
}

func LoadConfigFromEnv() Config {
	provider := os.Getenv("NEXUS_LLM_PROVIDER")
	if provider == "" {
		provider = "mock"
	}
	model := os.Getenv("NEXUS_LLM_MODEL")
	if model == "" {
		model = "mock-default"
	}
	ollama := os.Getenv("NEXUS_OLLAMA_BASE_URL")
	if ollama == "" {
		ollama = "http://localhost:11434"
	}
	cloudBase := os.Getenv("NEXUS_LLM_CLOUD_BASE_URL")
	if cloudBase == "" {
		cloudBase = "https://api.openai.com/v1"
	}
	cloudAPIKey := os.Getenv("NEXUS_LLM_CLOUD_API_KEY")
	return Config{
		Provider:      provider,
		Model:         model,
		OllamaBaseURL: ollama,
		CloudBaseURL:  cloudBase,
		CloudAPIKey:   cloudAPIKey,
		Timeout:       10 * time.Second,
	}
}
