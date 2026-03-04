package worker

import (
	"strings"

	"github.com/google/uuid"
	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
)

// ResolveIncidentID extrae el incident_id del evento como UUID.
func ResolveIncidentID(event opsdomain.StoredEvent) *uuid.UUID {
	if event.Envelope.Correlation.IncidentID != nil {
		if id, err := uuid.Parse(*event.Envelope.Correlation.IncidentID); err == nil {
			return &id
		}
	}
	if raw := strings.TrimSpace(AsString(event.Envelope.Payload["incident_id"])); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			return &id
		}
	}
	return nil
}

// ToAnySlice convierte un valor a []any.
func ToAnySlice(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	if arr, ok := v.([]interface{}); ok {
		return arr
	}
	return nil
}

// ToMap convierte un valor a map[string]any.
func ToMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	if m, ok := v.(map[string]interface{}); ok {
		out := map[string]any{}
		for k, val := range m {
			out[k] = val
		}
		return out
	}
	return map[string]any{}
}

// ToStringSlice convierte un valor a []string.
func ToStringSlice(v any) []string {
	raw := ToAnySlice(v)
	out := make([]string, 0, len(raw))
	for _, it := range raw {
		if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

// AsString convierte un valor a string.
func AsString(v any) string {
	s, _ := v.(string)
	return s
}

// AsInt convierte un valor a int con default.
func AsInt(v any, def int) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	default:
		return def
	}
}

// Ptr devuelve un puntero al string.
func Ptr(v string) *string {
	return &v
}
