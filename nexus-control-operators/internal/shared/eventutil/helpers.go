// Package eventutil provides helpers for decoding operator event payloads.
package eventutil

import (
	"math"
	"strings"

	"github.com/google/uuid"
	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
)

// ResolveIncidentID extracts the incident_id from an event via correlation or payload.
func ResolveIncidentID(event opsdomain.StoredEvent) string {
	if event.Envelope.Correlation.IncidentID != nil && *event.Envelope.Correlation.IncidentID != "" {
		return *event.Envelope.Correlation.IncidentID
	}
	if s := strings.TrimSpace(AsString(event.Envelope.Payload["incident_id"])); s != "" {
		return s
	}
	return ""
}

// ResolveIncidentUUID is like ResolveIncidentID but returns a *uuid.UUID.
func ResolveIncidentUUID(event opsdomain.StoredEvent) *uuid.UUID {
	raw := ResolveIncidentID(event)
	if raw == "" {
		return nil
	}
	if id, err := uuid.Parse(raw); err == nil {
		return &id
	}
	return nil
}

// ResolveToolName extracts the tool name from an event payload.
func ResolveToolName(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if tool := strings.TrimSpace(AsString(payload["tool_name"])); tool != "" {
		return tool
	}
	if tool := strings.TrimSpace(AsString(payload["tool_id"])); tool != "" {
		return tool
	}
	return ""
}

func AsString(v any) string {
	s, _ := v.(string)
	return s
}

func AsFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return 0
	}
}

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

func RoundFloat(v float64, decimals int) float64 {
	if decimals <= 0 {
		return math.Round(v)
	}
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
}

func ToAnySlice(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	if arr, ok := v.([]interface{}); ok {
		return arr
	}
	return nil
}

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

func ToStringMap(v any) map[string]string {
	raw := ToMap(v)
	out := map[string]string{}
	for k, val := range raw {
		if s := strings.TrimSpace(AsString(val)); s != "" {
			out[k] = s
		}
	}
	return out
}

func Ptr(v string) *string {
	return &v
}
