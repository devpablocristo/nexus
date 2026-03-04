package coreproxy

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
)

type EventstoreRepository struct {
	client        *Client
	defaultOrgID  uuid.UUID
	mu            sync.Mutex
	offsets       map[string]int64
	eventContract map[string]opsdomain.EventContract
}

func NewEventstoreRepository(client *Client, defaultOrgID uuid.UUID) *EventstoreRepository {
	return &EventstoreRepository{
		client:        client,
		defaultOrgID:  defaultOrgID,
		offsets:       map[string]int64{},
		eventContract: map[string]opsdomain.EventContract{},
	}
}

func (r *EventstoreRepository) Append(ctx context.Context, event opsdomain.Envelope, schemaValid bool, validationError *string) (opsdomain.StoredEvent, error) {
	payload := map[string]any{}
	for k, v := range event.Payload {
		payload[k] = v
	}
	payload["org_id"] = event.OrgID.String()
	payload["correlation"] = event.Correlation
	payload["actor"] = event.Actor
	payload["source"] = event.Source
	payload["occurred_at"] = event.OccurredAt.UTC().Format(time.RFC3339Nano)
	payload["schema_valid"] = schemaValid
	if validationError != nil {
		payload["validation_error"] = *validationError
	}

	req := map[string]any{
		"org_id":     event.OrgID.String(),
		"event_type": event.EventType,
		"payload":    payload,
	}
	var resp map[string]any
	if err := r.client.DoJSON(ctx, "POST", "/internal/operators/events/append", req, &resp); err != nil {
		return opsdomain.StoredEvent{}, err
	}

	seq := int64(0)
	if okVal, ok := resp["ok"].(bool); !ok || !okVal {
		return opsdomain.StoredEvent{}, fmt.Errorf("append failed")
	}

	// Sequence from SaaS list is monotonic ID, but append response only confirms accepted.
	// Use current monotonic time for immediate local ordering; consumers reconcile on fetch.
	seq = time.Now().UTC().UnixNano()

	return opsdomain.StoredEvent{
		Sequence:        seq,
		Envelope:        event,
		SchemaValid:     schemaValid,
		ValidationError: validationError,
		CreatedAt:       time.Now().UTC(),
	}, nil
}

type listEventsResponse struct {
	Items []struct {
		ID        int64          `json:"id"`
		EventType string         `json:"event_type"`
		Payload   map[string]any `json:"payload"`
		CreatedAt string         `json:"created_at"`
	} `json:"items"`
	NextCursor int64 `json:"next_cursor"`
}

func (r *EventstoreRepository) ListAfterSequence(ctx context.Context, orgID uuid.UUID, afterSequence int64, limit int) ([]opsdomain.StoredEvent, error) {
	events, err := r.ListGlobalAfterSequence(ctx, afterSequence, limit)
	if err != nil {
		return nil, err
	}
	out := make([]opsdomain.StoredEvent, 0, len(events))
	for _, ev := range events {
		if ev.Envelope.OrgID == orgID {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (r *EventstoreRepository) ListGlobalAfterSequence(ctx context.Context, afterSequence int64, limit int) ([]opsdomain.StoredEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	var resp listEventsResponse
	path := fmt.Sprintf("/internal/operators/events?cursor=%d&limit=%d", afterSequence, limit)
	if err := r.client.DoJSON(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]opsdomain.StoredEvent, 0, len(resp.Items))
	for _, item := range resp.Items {
		orgID := r.defaultOrgID
		if rawOrg, ok := item.Payload["org_id"].(string); ok {
			if parsed, err := uuid.Parse(rawOrg); err == nil {
				orgID = parsed
			}
		}
		createdAt, _ := time.Parse(time.RFC3339, item.CreatedAt)
		payload := map[string]any{}
		for k, v := range item.Payload {
			payload[k] = v
		}
		delete(payload, "org_id")
		out = append(out, opsdomain.StoredEvent{
			Sequence: item.ID,
			Envelope: opsdomain.Envelope{
				ID:         uuid.New(),
				EventType:  item.EventType,
				Version:    1,
				OccurredAt: createdAt,
				OrgID:      orgID,
				Payload:    payload,
			},
			SchemaValid: true,
			CreatedAt:   createdAt,
		})
	}
	return out, nil
}

func (r *EventstoreRepository) GetConsumerOffset(ctx context.Context, consumerGroup string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.offsets[consumerGroup], nil
}

func (r *EventstoreRepository) UpsertConsumerOffset(ctx context.Context, consumerGroup string, sequence int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.offsets[consumerGroup] = sequence
	return nil
}

func (r *EventstoreRepository) UpsertContract(ctx context.Context, in opsdomain.EventContract) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := in.EventType + ":" + fmt.Sprintf("%d", in.Version)
	r.eventContract[key] = in
	return nil
}

func (r *EventstoreRepository) GetContract(ctx context.Context, eventType string, version int) (opsdomain.EventContract, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := eventType + ":" + fmt.Sprintf("%d", version)
	if v, ok := r.eventContract[key]; ok {
		return v, nil
	}
	return opsdomain.EventContract{
		EventType: eventType,
		Version:   version,
		Enabled:   true,
		Schema:    map[string]any{},
	}, nil
}
