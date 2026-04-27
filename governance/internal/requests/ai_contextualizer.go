package requests

import (
	"context"
	"fmt"

	coreai "github.com/devpablocristo/core/ai/go"

	aitypes "github.com/devpablocristo/nexus/governance/internal/requests/ai_contextualizer"
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

// --- Implementación con core/ai/go ---

type coreAIContextualizer struct {
	provider coreai.Provider
}

// NewClaudeContextualizer crea un contextualizer usando core/ai/go.
func NewClaudeContextualizer(apiKey, model string) AIContextualizer {
	provider := coreai.NewProvider("anthropic", apiKey, model)
	return &coreAIContextualizer{provider: provider}
}

func (c *coreAIContextualizer) Summarize(ctx context.Context, input SummarizeInput) (string, bool, error) {
	userMsg := buildUserMessage(input)
	resp, err := c.provider.Chat(ctx, coreai.ChatRequest{
		SystemPrompt: systemPrompt,
		Messages:     []coreai.Message{{Role: "user", Content: userMsg}},
		MaxTokens:    300,
	})
	if err != nil {
		return fallbackSummary(input), true, fmt.Errorf("ai summarize: %w", err)
	}
	if resp.Text == "" {
		return fallbackSummary(input), true, fmt.Errorf("empty ai response")
	}
	return resp.Text, false, nil
}

const systemPrompt = `Sos un asistente de Nexus Review. Tu tarea es resumir una request de forma clara y concisa para que un humano aprobador pueda decidir en segundos.

Formato:
- Quién pide y qué pide (1 línea)
- Por qué se frenó (risk level + policy)
- Contexto relevante (1-2 líneas)
- Recomendación breve

Máximo 4 líneas. Español. Sin formato markdown.`

func buildUserMessage(input SummarizeInput) string {
	return fmt.Sprintf(
		"Requester: %s (%s)\nAcción: %s\nTarget: %s / %s\nMotivo: %s\nContexto: %s\nRisk: %s\nDecisión: %s (%s)",
		input.RequesterID, input.RequesterType,
		input.ActionType,
		input.TargetSystem, input.TargetResource,
		input.Reason, input.Context,
		input.RiskLevel,
		input.Decision, input.DecisionReason,
	)
}

func fallbackSummary(input SummarizeInput) string {
	return fmt.Sprintf(
		"Resumen no disponible. %s pide %s sobre %s. Motivo: %s",
		input.RequesterID, input.ActionType, input.TargetResource, input.Reason,
	)
}
