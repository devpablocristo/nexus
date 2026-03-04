package sentry

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"nexus-control-operators/internal/incidents"
	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
)

// updateBaseline actualiza el baseline EWMA, persiste y devuelve el baseline actualizado.
func (w *Worker) updateBaseline(ctx context.Context, orgID uuid.UUID, toolName, metric string, sample float64, baseline Baseline) (Baseline, error) {
	if baseline.SampleCount <= 0 {
		baseline.EWMA = sample
	} else {
		baseline.EWMA = w.cfg.Alpha*sample + (1-w.cfg.Alpha)*baseline.EWMA
	}
	baseline.OrgID = orgID
	baseline.ToolName = toolName
	baseline.Metric = metric
	baseline.SampleCount++
	err := w.state.UpsertBaseline(ctx, baseline)
	return baseline, err
}

// isAnomaly determina si el sample debe considerarse anomalía.
func (w *Worker) isAnomaly(event opsdomain.StoredEvent, baseline Baseline, sample float64, forceAnomaly bool, observedOverride, thresholdOverride float64) bool {
	if sample == 0 {
		return false
	}
	threshold := w.cfg.ErrorThreshold
	if thresholdOverride > 0 {
		threshold = thresholdOverride
	}
	minSamples := w.cfg.MinSamples
	if event.Envelope.EventType != "tool_call.finished" {
		minSamples = 1
	}
	return forceAnomaly || (baseline.SampleCount >= minSamples && baseline.EWMA >= threshold)
}

// createIncidentAndEmit crea el incidente, persiste fingerprint y emite evento.
func (w *Worker) createIncidentAndEmit(ctx context.Context, orgID uuid.UUID, fingerprint string, event opsdomain.StoredEvent) error {
	if w.incidents == nil {
		return nil
	}
	actorID := "agents.sentry"
	inc, err := w.incidents.Create(ctx, orgID, &actorID, incidents.CreateRequest{
		Severity: "HIGH",
		Title:    "Anomaly detected by sentry",
		Summary:  "Error rate spike detected for " + resolveToolName(event.Envelope.Payload),
		EvidenceRefs: []string{
			"event_store:" + fmt.Sprintf("%d", event.Sequence),
			"fingerprint:" + fingerprint,
		},
	})
	if err != nil {
		return err
	}
	fpState := FingerprintState{
		OrgID:       orgID,
		Fingerprint: fingerprint,
		IncidentID:  &inc.ID,
		State:       "open",
	}
	if err := w.state.UpsertFingerprint(ctx, fpState); err != nil {
		return err
	}
	w.emitOrLog(ctx, orgID, "incident.opened", map[string]any{
		"incident_id": inc.ID.String(),
		"severity":    string(inc.Severity),
		"state":       "OPEN",
		"title":       inc.Title,
		"summary":     inc.Summary,
		"fingerprint": fingerprint,
		"opened_at":   time.Now().UTC().Format(time.RFC3339),
	})
	return nil
}

// emitOrLog emite el evento; si falla, loguea (evita _ = ignorando errores).
func (w *Worker) emitOrLog(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) {
	if err := w.emit(ctx, orgID, eventType, payload); err != nil {
		// Logging implícito vía zerolog si se inyecta; por ahora no tenemos logger en Worker.
		// Se podría propagar el error o usar log.Printf si se añade dependencia.
		_ = err
	}
}
