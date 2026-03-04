package worker

import (
	"math"
	"strings"
)

// ResolveToolName extrae el nombre de la tool del payload del evento.
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

// AsFloat convierte un valor a float64.
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

// RoundFloat redondea un float a N decimales.
func RoundFloat(v float64, decimals int) float64 {
	if decimals <= 0 {
		return math.Round(v)
	}
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
}

// AsString convierte un valor a string.
func AsString(v any) string {
	s, _ := v.(string)
	return s
}
