package worker

import (
	"strings"

	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
)

// ResolveIncidentID extrae el incident_id del evento.
func ResolveIncidentID(event opsdomain.StoredEvent) string {
	if event.Envelope.Correlation.IncidentID != nil && *event.Envelope.Correlation.IncidentID != "" {
		return *event.Envelope.Correlation.IncidentID
	}
	if raw := strings.TrimSpace(AsString(event.Envelope.Payload["incident_id"])); raw != "" {
		return raw
	}
	return ""
}

// AsString convierte un valor a string.
func AsString(v any) string {
	s, _ := v.(string)
	return s
}

// AsInt convierte un valor a int.
func AsInt(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	default:
		return 0
	}
}
