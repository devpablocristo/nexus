package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	aitypes "github.com/devpablocristo/nexus/v3/review/internal/requests/ai_contextualizer"
)

// SummarizeInput re-exporta el tipo del adapter para uso externo.
type SummarizeInput = aitypes.SummarizeInput

// AIContextualizer define el port para el contextualizer de AI.
type AIContextualizer interface {
	Summarize(ctx context.Context, input SummarizeInput) (summary string, degraded bool, err error)
}

// --- Stub (tests y fallback) ---

type stubContextualizer struct{}

func NewStubContextualizer() AIContextualizer {
	return &stubContextualizer{}
}

func (s *stubContextualizer) Summarize(_ context.Context, input SummarizeInput) (string, bool, error) {
	return "Resumen no disponible (modo fallback). Requester: " + input.RequesterID +
		", Acción: " + input.ActionType + " sobre " + input.TargetResource +
		". Motivo: " + input.Reason, true, nil
}

// --- Claude (HTTP directo, sin SDK externo) ---
// Patrón de v1/ai-runtime: HTTP a https://api.anthropic.com/v1/messages

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"
const anthropicVersion = "2023-06-01"

type claudeContextualizer struct {
	apiKey  string
	model   string
	timeout time.Duration
	client  *http.Client
}

func NewClaudeContextualizer(apiKey, model string, timeout time.Duration) AIContextualizer {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	return &claudeContextualizer{
		apiKey:  apiKey,
		model:   model,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *claudeContextualizer) Summarize(ctx context.Context, input SummarizeInput) (string, bool, error) {
	if c.apiKey == "" {
		return fallbackSummary(input), true, nil
	}

	userMsg := buildUserMessage(input)
	body := map[string]any{
		"model":      c.model,
		"max_tokens": 300,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userMsg},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fallbackSummary(input), true, fmt.Errorf("marshal request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, anthropicAPIURL, bytes.NewReader(payload))
	if err != nil {
		return fallbackSummary(input), true, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.client.Do(req)
	if err != nil {
		return fallbackSummary(input), true, fmt.Errorf("call anthropic: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fallbackSummary(input), true, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fallbackSummary(input), true, fmt.Errorf("anthropic returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result aitypes.AnthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fallbackSummary(input), true, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(result.Content) == 0 || result.Content[0].Text == "" {
		return fallbackSummary(input), true, fmt.Errorf("empty response from anthropic")
	}

	return result.Content[0].Text, false, nil
}

const systemPrompt = `Sos un asistente de Nexus Review. Tu tarea es resumir una request de forma clara y concisa para que un humano aprobador pueda decidir en segundos.

Formato:
- Quién pide y qué pide (1 línea)
- Por qué se frenó (risk level + policy)
- Contexto relevante (1-2 líneas)
- Recomendación breve

Máximo 4 líneas. Español. Sin formato markdown.`

func buildUserMessage(input SummarizeInput) string {
	msg := fmt.Sprintf(
		"Requester: %s (%s)\nAcción: %s\nTarget: %s / %s\nMotivo: %s\nContexto: %s\nRisk: %s\nDecisión: %s (%s)",
		input.RequesterID, input.RequesterType,
		input.ActionType,
		input.TargetSystem, input.TargetResource,
		input.Reason, input.Context,
		input.RiskLevel,
		input.Decision, input.DecisionReason,
	)
	if len(input.Params) > 0 {
		paramsJSON, err := json.Marshal(input.Params)
		if err != nil {
			slog.Error("marshal params for AI prompt", "error", err)
		} else {
			msg += "\nParams: " + string(paramsJSON)
		}
	}
	return msg
}

func fallbackSummary(input SummarizeInput) string {
	return fmt.Sprintf(
		"%s (%s) quiere %s sobre %s/%s. Risk: %s. %s. Motivo: %s",
		input.RequesterID, input.RequesterType,
		input.ActionType, input.TargetSystem, input.TargetResource,
		input.RiskLevel, input.DecisionReason, input.Reason,
	)
}
