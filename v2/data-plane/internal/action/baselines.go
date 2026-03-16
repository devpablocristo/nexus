package action

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	sharedpostgres "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/postgres"
	actionrisk "nexus/v2/data-plane/internal/action/risk"
	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

type RiskContextProvider interface {
	ContextFor(ctx context.Context, req CreateRequest, resource actiondomain.ProtectedResource, now time.Time) (actionrisk.Context, error)
	ObserveAction(ctx context.Context, item actiondomain.Action) error
	RefreshAll(ctx context.Context) error
	Start(ctx context.Context, interval time.Duration, logger *slog.Logger)
}

type OpenIncidentReader interface {
	CountOpenByResource(ctx context.Context, resourceID string) (int, error)
}

type RiskBaselineStore interface {
	ListBaselines(ctx context.Context, scopeType actionrisk.ScopeType, scopeID string) ([]actionrisk.Baseline, error)
	UpsertBaselines(ctx context.Context, items []actionrisk.Baseline) error
	GetKnownDestination(ctx context.Context, resourceID, destination string) (*actionrisk.KnownDestination, error)
	UpsertKnownDestination(ctx context.Context, item actionrisk.KnownDestination) error
}

type InMemoryRiskBaselineStore struct {
	mu                sync.RWMutex
	baselines         map[string]actionrisk.Baseline
	knownDestinations map[string]actionrisk.KnownDestination
}

func NewInMemoryRiskBaselineStore() *InMemoryRiskBaselineStore {
	return &InMemoryRiskBaselineStore{
		baselines:         make(map[string]actionrisk.Baseline),
		knownDestinations: make(map[string]actionrisk.KnownDestination),
	}
}

func (s *InMemoryRiskBaselineStore) ListBaselines(_ context.Context, scopeType actionrisk.ScopeType, scopeID string) ([]actionrisk.Baseline, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]actionrisk.Baseline, 0)
	prefix := baselineKey(scopeType, scopeID, "")
	for key, item := range s.baselines {
		if strings.HasPrefix(key, prefix) {
			items = append(items, item)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Metric < items[j].Metric })
	return items, nil
}

func (s *InMemoryRiskBaselineStore) UpsertBaselines(_ context.Context, items []actionrisk.Baseline) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range items {
		s.baselines[baselineKey(item.ScopeType, item.ScopeID, item.Metric)] = item
	}
	return nil
}

func (s *InMemoryRiskBaselineStore) GetKnownDestination(_ context.Context, resourceID, destination string) (*actionrisk.KnownDestination, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.knownDestinations[knownDestinationKey(resourceID, destination)]
	if !ok {
		return nil, nil
	}
	copy := item
	return &copy, nil
}

func (s *InMemoryRiskBaselineStore) UpsertKnownDestination(_ context.Context, item actionrisk.KnownDestination) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.knownDestinations[knownDestinationKey(item.ResourceID, item.Destination)] = item
	return nil
}

type PostgresRiskBaselineStore struct {
	db *sharedpostgres.DB
}

func NewPostgresRiskBaselineStore(db *sharedpostgres.DB) *PostgresRiskBaselineStore {
	return &PostgresRiskBaselineStore{db: db}
}

func (s *PostgresRiskBaselineStore) ListBaselines(ctx context.Context, scopeType actionrisk.ScopeType, scopeID string) ([]actionrisk.Baseline, error) {
	rows, err := s.db.Pool().Query(ctx, `
		SELECT scope_type, scope_id, metric, avg, stddev, p95, sample_size, window_days, computed_at
		FROM baselines
		WHERE scope_type = $1 AND scope_id = $2
		ORDER BY metric ASC
	`, string(scopeType), scopeID)
	if err != nil {
		return nil, fmt.Errorf("list baselines: %w", err)
	}
	defer rows.Close()

	items := make([]actionrisk.Baseline, 0)
	for rows.Next() {
		var item actionrisk.Baseline
		if err := rows.Scan(
			&item.ScopeType,
			&item.ScopeID,
			&item.Metric,
			&item.Avg,
			&item.Stddev,
			&item.P95,
			&item.SampleSize,
			&item.WindowDays,
			&item.ComputedAt,
		); err != nil {
			return nil, fmt.Errorf("scan baseline: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate baselines: %w", err)
	}
	return items, nil
}

func (s *PostgresRiskBaselineStore) UpsertBaselines(ctx context.Context, items []actionrisk.Baseline) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin baseline tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO baselines (scope_type, scope_id, metric, avg, stddev, p95, sample_size, window_days, computed_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (scope_type, scope_id, metric)
			DO UPDATE SET
				avg = EXCLUDED.avg,
				stddev = EXCLUDED.stddev,
				p95 = EXCLUDED.p95,
				sample_size = EXCLUDED.sample_size,
				window_days = EXCLUDED.window_days,
				computed_at = EXCLUDED.computed_at
		`,
			string(item.ScopeType),
			item.ScopeID,
			string(item.Metric),
			item.Avg,
			item.Stddev,
			item.P95,
			item.SampleSize,
			item.WindowDays,
			item.ComputedAt,
		); err != nil {
			return fmt.Errorf("upsert baseline: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit baseline tx: %w", err)
	}
	return nil
}

func (s *PostgresRiskBaselineStore) GetKnownDestination(ctx context.Context, resourceID, destination string) (*actionrisk.KnownDestination, error) {
	row := s.db.Pool().QueryRow(ctx, `
		SELECT resource_id, destination, first_seen, last_seen, tx_count, is_internal
		FROM known_destinations
		WHERE resource_id = $1 AND destination = $2
	`, resourceID, destination)
	var item actionrisk.KnownDestination
	if err := row.Scan(&item.ResourceID, &item.Destination, &item.FirstSeen, &item.LastSeen, &item.TxCount, &item.IsInternal); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get known destination: %w", err)
	}
	return &item, nil
}

func (s *PostgresRiskBaselineStore) UpsertKnownDestination(ctx context.Context, item actionrisk.KnownDestination) error {
	_, err := s.db.Pool().Exec(ctx, `
		INSERT INTO known_destinations (resource_id, destination, first_seen, last_seen, tx_count, is_internal)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (resource_id, destination)
		DO UPDATE SET
			first_seen = LEAST(known_destinations.first_seen, EXCLUDED.first_seen),
			last_seen = GREATEST(known_destinations.last_seen, EXCLUDED.last_seen),
			tx_count = GREATEST(known_destinations.tx_count, EXCLUDED.tx_count),
			is_internal = EXCLUDED.is_internal
	`,
		item.ResourceID,
		item.Destination,
		item.FirstSeen,
		item.LastSeen,
		item.TxCount,
		item.IsInternal,
	)
	if err != nil {
		return fmt.Errorf("upsert known destination: %w", err)
	}
	return nil
}

type HistoricalRiskContextProvider struct {
	repo      Repository
	store     RiskBaselineStore
	incidents OpenIncidentReader
}

func NewHistoricalRiskContextProvider(repo Repository, store RiskBaselineStore) *HistoricalRiskContextProvider {
	return &HistoricalRiskContextProvider{repo: repo, store: store}
}

func (p *HistoricalRiskContextProvider) WithIncidentReader(reader OpenIncidentReader) *HistoricalRiskContextProvider {
	p.incidents = reader
	return p
}

func (p *HistoricalRiskContextProvider) ContextFor(ctx context.Context, req CreateRequest, resource actiondomain.ProtectedResource, now time.Time) (actionrisk.Context, error) {
	resourceBaselines, err := p.baselinesByMetric(ctx, actionrisk.ScopeTypeResource, resource.ID)
	if err != nil {
		return actionrisk.Context{}, err
	}
	actorBaselines, err := p.baselinesByMetric(ctx, actionrisk.ScopeTypeActor, req.ProposedBy.ID)
	if err != nil {
		return actionrisk.Context{}, err
	}
	recentActorCount := 0
	if req.ProposedBy.ID != "" {
		history, err := p.repo.ListHistory(ctx, HistoryFilters{
			ActorID: req.ProposedBy.ID,
			Since:   now.Add(-30 * time.Minute),
			Before:  now,
		})
		if err != nil {
			return actionrisk.Context{}, err
		}
		recentActorCount = len(history) + 1
	}
	var previousDecision *actiondomain.RiskDecision
	if req.ResourceID != "" {
		history, err := p.repo.ListHistory(ctx, HistoryFilters{
			ResourceID: req.ResourceID,
			ActorID:    req.ProposedBy.ID,
			Before:     now,
			Limit:      1,
		})
		if err != nil {
			return actionrisk.Context{}, err
		}
		if len(history) == 1 {
			decision := history[0].Risk.RecommendedDecision
			previousDecision = &decision
		}
	}
	var knownDestination *actionrisk.KnownDestination
	if destination := destinationFromRequest(req.ActionType, req.Payload); destination != "" {
		item, err := p.store.GetKnownDestination(ctx, req.ResourceID, destination)
		if err != nil {
			return actionrisk.Context{}, err
		}
		if item != nil {
			item.Confidence = destinationConfidence(item.LastSeen, now)
			item.ObservedDays = now.Sub(item.FirstSeen).Hours() / 24
			knownDestination = item
		}
	}
	openIncidents := 0
	if p.incidents != nil && req.ResourceID != "" {
		count, err := p.incidents.CountOpenByResource(ctx, req.ResourceID)
		if err != nil {
			return actionrisk.Context{}, err
		}
		openIncidents = count
	}
	return actionrisk.Context{
		Now:                now,
		ResourceBaselines:  resourceBaselines,
		ActorBaselines:     actorBaselines,
		KnownDestination:   knownDestination,
		RecentActorCount30: recentActorCount,
		OpenIncidentCount:  openIncidents,
		VerifiedActor:      verifiedActorFromMetadata(req.Metadata),
		PreviousDecision:   previousDecision,
	}, nil
}

func (p *HistoricalRiskContextProvider) ObserveAction(ctx context.Context, item actiondomain.Action) error {
	if item.ResourceID != "" {
		if err := p.refreshResourceScope(ctx, item.ResourceID, time.Now().UTC()); err != nil {
			return err
		}
	}
	if item.ProposedBy.ID != "" {
		if err := p.refreshActorScope(ctx, item.ProposedBy.ID, time.Now().UTC()); err != nil {
			return err
		}
	}
	destination := destinationFromAction(item)
	if destination == "" {
		return nil
	}
	history, err := p.repo.ListHistory(ctx, HistoryFilters{
		ResourceID: item.ResourceID,
		Since:      time.Now().UTC().Add(-30 * 24 * time.Hour),
	})
	if err != nil {
		return err
	}
	known := summarizeKnownDestination(item.ResourceID, destination, destinationIsInternal(item.Type), history)
	if known == nil {
		return nil
	}
	return p.store.UpsertKnownDestination(ctx, *known)
}

func (p *HistoricalRiskContextProvider) RefreshAll(ctx context.Context) error {
	now := time.Now().UTC()
	resourceIDs, err := p.repo.ListDistinctResourceIDs(ctx, now.Add(-30*24*time.Hour))
	if err != nil {
		return err
	}
	for _, resourceID := range resourceIDs {
		if err := p.refreshResourceScope(ctx, resourceID, now); err != nil {
			return err
		}
	}
	actorIDs, err := p.repo.ListDistinctActorIDs(ctx, now.Add(-30*24*time.Hour))
	if err != nil {
		return err
	}
	for _, actorID := range actorIDs {
		if err := p.refreshActorScope(ctx, actorID, now); err != nil {
			return err
		}
	}
	return nil
}

func (p *HistoricalRiskContextProvider) Start(ctx context.Context, interval time.Duration, logger *slog.Logger) {
	if interval <= 0 {
		interval = time.Hour
	}
	if logger == nil {
		logger = slog.Default()
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := p.RefreshAll(context.Background()); err != nil {
					logger.Error("risk baseline refresh failed", "error", err)
				}
			}
		}
	}()
}

func (p *HistoricalRiskContextProvider) baselinesByMetric(ctx context.Context, scopeType actionrisk.ScopeType, scopeID string) (map[actionrisk.Metric]actionrisk.Baseline, error) {
	items := make(map[actionrisk.Metric]actionrisk.Baseline)
	if strings.TrimSpace(scopeID) == "" {
		return items, nil
	}
	baselines, err := p.store.ListBaselines(ctx, scopeType, scopeID)
	if err != nil {
		return nil, err
	}
	for _, item := range baselines {
		items[item.Metric] = item
	}
	return items, nil
}

func (p *HistoricalRiskContextProvider) refreshResourceScope(ctx context.Context, resourceID string, now time.Time) error {
	history, err := p.repo.ListHistory(ctx, HistoryFilters{
		ResourceID: resourceID,
		Since:      now.Add(-30 * 24 * time.Hour),
		Before:     now,
	})
	if err != nil {
		return err
	}
	items := computeResourceBaselines(resourceID, history, now)
	if err := p.store.UpsertBaselines(ctx, items); err != nil {
		return err
	}
	knownDestinations := summarizeKnownDestinations(resourceID, history, now)
	for _, item := range knownDestinations {
		if err := p.store.UpsertKnownDestination(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (p *HistoricalRiskContextProvider) refreshActorScope(ctx context.Context, actorID string, now time.Time) error {
	history, err := p.repo.ListHistory(ctx, HistoryFilters{
		ActorID: actorID,
		Since:   now.Add(-30 * 24 * time.Hour),
		Before:  now,
	})
	if err != nil {
		return err
	}
	return p.store.UpsertBaselines(ctx, computeActorBaselines(actorID, history, now))
}

func computeResourceBaselines(resourceID string, history []actiondomain.Action, now time.Time) []actionrisk.Baseline {
	dailyCount := make(map[string]float64)
	dailyVolume := make(map[string]float64)
	dailyAmounts := make(map[string][]float64)
	dailyDestinations := make(map[string]map[string]struct{})
	for _, item := range history {
		day := item.CreatedAt.UTC().Format("2006-01-02")
		dailyCount[day]++
		amount := amountFromAction(item)
		if amount > 0 {
			dailyVolume[day] += amount
			dailyAmounts[day] = append(dailyAmounts[day], amount)
		}
		destination := destinationFromAction(item)
		if destination != "" {
			if dailyDestinations[day] == nil {
				dailyDestinations[day] = make(map[string]struct{})
			}
			dailyDestinations[day][destination] = struct{}{}
		}
	}
	valuesCount := valuesFromMap(dailyCount)
	valuesVolume := valuesFromMap(dailyVolume)
	valuesAvgAmount := make([]float64, 0, len(dailyAmounts))
	for _, amounts := range dailyAmounts {
		valuesAvgAmount = append(valuesAvgAmount, average(amounts))
	}
	valuesUniqueDestinations := make([]float64, 0, len(dailyDestinations))
	for _, set := range dailyDestinations {
		valuesUniqueDestinations = append(valuesUniqueDestinations, float64(len(set)))
	}
	return []actionrisk.Baseline{
		buildBaseline(actionrisk.ScopeTypeResource, resourceID, actionrisk.MetricDailyTxCount, valuesCount, 30, now),
		buildBaseline(actionrisk.ScopeTypeResource, resourceID, actionrisk.MetricDailyVolume, valuesVolume, 30, now),
		buildBaseline(actionrisk.ScopeTypeResource, resourceID, actionrisk.MetricAvgTxAmount, valuesAvgAmount, 30, now),
		buildBaseline(actionrisk.ScopeTypeResource, resourceID, actionrisk.MetricUniqueDestinations, valuesUniqueDestinations, 30, now),
	}
}

func computeActorBaselines(actorID string, history []actiondomain.Action, now time.Time) []actionrisk.Baseline {
	dailyActionCount := make(map[string]float64)
	buckets30m := make(map[time.Time]float64)
	hours := make([]float64, 0, len(history))
	for _, item := range history {
		day := item.CreatedAt.UTC().Format("2006-01-02")
		dailyActionCount[day]++
		bucket := item.CreatedAt.UTC().Truncate(30 * time.Minute)
		buckets30m[bucket]++
		hours = append(hours, float64(item.CreatedAt.UTC().Hour()))
	}
	return []actionrisk.Baseline{
		buildBaseline(actionrisk.ScopeTypeActor, actorID, actionrisk.MetricDailyActionCount, valuesFromMap(dailyActionCount), 30, now),
		buildBaseline(actionrisk.ScopeTypeActor, actorID, actionrisk.MetricActions30mCount, valuesFromTimeMap(buckets30m), 30, now),
		buildBaseline(actionrisk.ScopeTypeActor, actorID, actionrisk.MetricTypicalHours, hours, 30, now),
	}
}

func buildBaseline(scopeType actionrisk.ScopeType, scopeID string, metric actionrisk.Metric, values []float64, windowDays int, now time.Time) actionrisk.Baseline {
	avg, stddev, p95 := summarize(values)
	return actionrisk.Baseline{
		ScopeType:  scopeType,
		ScopeID:    scopeID,
		Metric:     metric,
		Avg:        avg,
		Stddev:     stddev,
		P95:        p95,
		SampleSize: len(values),
		WindowDays: windowDays,
		ComputedAt: now,
	}
}

func summarizeKnownDestinations(resourceID string, history []actiondomain.Action, now time.Time) []actionrisk.KnownDestination {
	items := make(map[string]*actionrisk.KnownDestination)
	for _, item := range history {
		destination := destinationFromAction(item)
		if destination == "" {
			continue
		}
		entry, ok := items[destination]
		if !ok {
			entry = &actionrisk.KnownDestination{
				ResourceID:  resourceID,
				Destination: destination,
				FirstSeen:   item.CreatedAt,
				LastSeen:    item.CreatedAt,
				TxCount:     0,
				IsInternal:  destinationIsInternal(item.Type),
			}
			items[destination] = entry
		}
		if item.CreatedAt.Before(entry.FirstSeen) {
			entry.FirstSeen = item.CreatedAt
		}
		if item.CreatedAt.After(entry.LastSeen) {
			entry.LastSeen = item.CreatedAt
		}
		entry.TxCount++
	}
	out := make([]actionrisk.KnownDestination, 0, len(items))
	for _, item := range items {
		item.Confidence = destinationConfidence(item.LastSeen, now)
		item.ObservedDays = now.Sub(item.FirstSeen).Hours() / 24
		out = append(out, *item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Destination < out[j].Destination })
	return out
}

func summarizeKnownDestination(resourceID, destination string, isInternal bool, history []actiondomain.Action) *actionrisk.KnownDestination {
	for _, item := range summarizeKnownDestinations(resourceID, history, time.Now().UTC()) {
		if item.Destination == destination {
			item.IsInternal = isInternal
			return &item
		}
	}
	return nil
}

func valuesFromMap(items map[string]float64) []float64 {
	values := make([]float64, 0, len(items))
	for _, value := range items {
		values = append(values, value)
	}
	sort.Float64s(values)
	return values
}

func valuesFromTimeMap(items map[time.Time]float64) []float64 {
	values := make([]float64, 0, len(items))
	for _, value := range items {
		values = append(values, value)
	}
	sort.Float64s(values)
	return values
}

func summarize(values []float64) (float64, float64, float64) {
	if len(values) == 0 {
		return 0, 0, 0
	}
	avg := average(values)
	variance := 0.0
	for _, value := range values {
		diff := value - avg
		variance += diff * diff
	}
	stddev := math.Sqrt(variance / float64(len(values)))
	p95 := percentile(values, 0.95)
	return roundTwo(avg), roundTwo(stddev), roundTwo(p95)
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copied := append([]float64(nil), values...)
	sort.Float64s(copied)
	index := int(math.Ceil(float64(len(copied))*p)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(copied) {
		index = len(copied) - 1
	}
	return copied[index]
}

func destinationConfidence(lastSeen, now time.Time) float64 {
	if lastSeen.IsZero() {
		return 0
	}
	days := now.Sub(lastSeen).Hours() / 24
	return roundTwo(math.Exp(-days / 30.0))
}

func baselineKey(scopeType actionrisk.ScopeType, scopeID string, metric actionrisk.Metric) string {
	return string(scopeType) + "|" + scopeID + "|" + string(metric)
}

func knownDestinationKey(resourceID, destination string) string {
	return resourceID + "|" + destination
}

func verifiedActorFromMetadata(metadata map[string]any) bool {
	if len(metadata) == 0 {
		return false
	}
	return boolField(metadata, "verified_actor") || boolField(metadata, "ip_known")
}

func boolField(metadata map[string]any, key string) bool {
	value, ok := metadata[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func amountFromAction(item actiondomain.Action) float64 {
	switch item.Type {
	case actiondomain.ActionTypeWithdrawal:
		var payload actiondomain.WithdrawalPayload
		if err := json.Unmarshal(item.Payload, &payload); err == nil {
			value, _ := strconvParseFloat(strings.TrimSpace(payload.Amount))
			return value
		}
	case actiondomain.ActionTypeTreasuryTransfer:
		var payload actiondomain.TreasuryTransferPayload
		if err := json.Unmarshal(item.Payload, &payload); err == nil {
			value, _ := strconvParseFloat(strings.TrimSpace(payload.Amount))
			return value
		}
	case actiondomain.ActionTypeHotToColdMove:
		var payload actiondomain.HotToColdMovePayload
		if err := json.Unmarshal(item.Payload, &payload); err == nil {
			value, _ := strconvParseFloat(strings.TrimSpace(payload.Amount))
			return value
		}
	}
	return 0
}

func destinationFromAction(item actiondomain.Action) string {
	switch item.Type {
	case actiondomain.ActionTypeWithdrawal:
		var payload actiondomain.WithdrawalPayload
		if err := json.Unmarshal(item.Payload, &payload); err == nil {
			return normalizeRiskDestination(payload.DestinationAddress)
		}
	case actiondomain.ActionTypeTreasuryTransfer:
		var payload actiondomain.TreasuryTransferPayload
		if err := json.Unmarshal(item.Payload, &payload); err == nil {
			return normalizeRiskDestination(payload.ToAccount)
		}
	case actiondomain.ActionTypeHotToColdMove:
		var payload actiondomain.HotToColdMovePayload
		if err := json.Unmarshal(item.Payload, &payload); err == nil {
			return normalizeRiskDestination(payload.ToWallet)
		}
	}
	return ""
}

func destinationFromRequest(actionType actiondomain.ActionType, payload json.RawMessage) string {
	switch actionType {
	case actiondomain.ActionTypeWithdrawal:
		var item actiondomain.WithdrawalPayload
		if err := json.Unmarshal(payload, &item); err == nil {
			return normalizeRiskDestination(item.DestinationAddress)
		}
	case actiondomain.ActionTypeTreasuryTransfer:
		var item actiondomain.TreasuryTransferPayload
		if err := json.Unmarshal(payload, &item); err == nil {
			return normalizeRiskDestination(item.ToAccount)
		}
	case actiondomain.ActionTypeHotToColdMove:
		var item actiondomain.HotToColdMovePayload
		if err := json.Unmarshal(payload, &item); err == nil {
			return normalizeRiskDestination(item.ToWallet)
		}
	}
	return ""
}

func destinationIsInternal(actionType actiondomain.ActionType) bool {
	return actionType != actiondomain.ActionTypeWithdrawal
}

func normalizeRiskDestination(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func roundTwo(value float64) float64 {
	return math.Round(value*100) / 100
}

func strconvParseFloat(value string) (float64, error) {
	return strconv.ParseFloat(value, 64)
}
