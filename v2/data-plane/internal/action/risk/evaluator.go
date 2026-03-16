package risk

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

const (
	factorAmountAnomaly    = "amount_anomaly"
	factorVelocitySpike    = "velocity_spike"
	factorNewDestination   = "new_destination"
	factorOffHours         = "off_hours"
	factorActorDeviation   = "actor_deviation"
	factorRecentIncident   = "recent_incident"
	factorKnownDestination = "known_destination"
	factorWithinBaseline   = "within_baseline"
	factorBusinessHours    = "business_hours"
	factorVerifiedActor    = "verified_actor"
)

type Evaluator struct{}

type Input struct {
	ActionType actiondomain.ActionType
	Resource   actiondomain.ProtectedResource
	Actor      actiondomain.ActorRef
	Payload    json.RawMessage
	Metadata   map[string]any
	Now        time.Time
	Context    Context
}

type profile struct {
	Name           string
	Version        int
	Bands          [4]float64
	Weights        map[string]float64
	Amplifications []interactionProfile
	Attenuations   []interactionProfile
}

type interactionProfile struct {
	Factors    []string
	Multiplier float64
	Summary    string
}

type signals struct {
	Now                   time.Time
	Amount                float64
	HasAmount             bool
	Destination           string
	HasDestination        bool
	DestinationIsInternal bool
	RecentActorCount30    int
	OpenIncidentCount     int
	VerifiedActor         bool
	PreviousDecision      *actiondomain.RiskDecision
}

func (Evaluator) Evaluate(input Input) (actiondomain.RiskAssessment, error) {
	signals, err := deriveSignals(input)
	if err != nil {
		return actiondomain.RiskAssessment{}, err
	}
	profile := balancedProfile()

	factors := evaluateFactors(profile, input.Resource, signals, input.Context)
	riskPressure, safetyPressure := accumulatePressures(factors)
	amplifications := activeInteractions(profile.Amplifications, factors)
	attenuations := activeInteractions(profile.Attenuations, factors)

	riskMultiplier := capRiskMultiplier(amplifications)
	safetyMultiplier := combinedMultiplier(attenuations)
	riskPressure = roundTwo(riskPressure * riskMultiplier)
	safetyPressure = roundTwo(safetyPressure * safetyMultiplier)

	rawScore := roundTwo(riskPressure - safetyPressure)
	decisionScore := clampZeroToOne(rawScore)
	recommended := classifyDecision(profile.Bands, decisionScore)
	recommended = applyHysteresis(recommended, signals.PreviousDecision, profile.Bands, decisionScore)
	score := int(math.Round(decisionScore * 100))

	return actiondomain.RiskAssessment{
		Level:               classifyLevel(score),
		Score:               score,
		Summary:             buildSummary(profile, recommended, factors),
		Profile:             actiondomain.RiskProfileRef{Name: profile.Name, Version: profile.Version},
		RiskPressure:        riskPressure,
		SafetyPressure:      safetyPressure,
		RawScore:            rawScore,
		DecisionScore:       roundTwo(decisionScore),
		RecommendedDecision: recommended,
		Factors:             factors,
		Amplifications:      toInteractions(amplifications),
		Attenuations:        toInteractions(attenuations),
	}, nil
}

func balancedProfile() profile {
	return profile{
		Name:    "balanced",
		Version: 1,
		Bands:   [4]float64{0.20, 0.40, 0.60, 0.80},
		Weights: map[string]float64{
			factorAmountAnomaly:    0.15,
			factorVelocitySpike:    0.20,
			factorNewDestination:   0.15,
			factorOffHours:         0.10,
			factorActorDeviation:   0.20,
			factorRecentIncident:   0.10,
			factorKnownDestination: 0.20,
			factorWithinBaseline:   0.15,
			factorBusinessHours:    0.10,
			factorVerifiedActor:    0.15,
		},
		Amplifications: []interactionProfile{
			{Factors: []string{factorAmountAnomaly, factorVelocitySpike}, Multiplier: 1.5, Summary: "amount anomaly combined with velocity spike"},
			{Factors: []string{factorNewDestination, factorActorDeviation}, Multiplier: 2.0, Summary: "new destination combined with actor deviation"},
			{Factors: []string{factorAmountAnomaly, factorNewDestination, factorOffHours}, Multiplier: 2.5, Summary: "amount anomaly, new destination and off-hours together"},
		},
		Attenuations: []interactionProfile{
			{Factors: []string{factorKnownDestination, factorVerifiedActor}, Multiplier: 1.5, Summary: "known destination with verified actor"},
			{Factors: []string{factorWithinBaseline, factorBusinessHours}, Multiplier: 1.3, Summary: "within baseline during business hours"},
		},
	}
}

func deriveSignals(input Input) (signals, error) {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	out := signals{
		Now:                now,
		RecentActorCount30: input.Context.RecentActorCount30,
		OpenIncidentCount:  input.Context.OpenIncidentCount,
		VerifiedActor:      actorVerified(input.Actor, input.Metadata, input.Context),
		PreviousDecision:   input.Context.PreviousDecision,
	}

	switch input.ActionType {
	case actiondomain.ActionTypeWithdrawal:
		var item actiondomain.WithdrawalPayload
		if err := json.Unmarshal(input.Payload, &item); err != nil {
			return signals{}, fmt.Errorf("decode withdrawal payload for risk evaluation: %w", err)
		}
		out.HasAmount = strings.TrimSpace(item.Amount) != ""
		out.Amount = parseAmount(item.Amount)
		out.HasDestination = strings.TrimSpace(item.DestinationAddress) != ""
		out.Destination = normalizeDestination(item.DestinationAddress)
		out.DestinationIsInternal = false
	case actiondomain.ActionTypeTreasuryTransfer:
		var item actiondomain.TreasuryTransferPayload
		if err := json.Unmarshal(input.Payload, &item); err != nil {
			return signals{}, fmt.Errorf("decode treasury transfer payload for risk evaluation: %w", err)
		}
		out.HasAmount = strings.TrimSpace(item.Amount) != ""
		out.Amount = parseAmount(item.Amount)
		out.HasDestination = strings.TrimSpace(item.ToAccount) != ""
		out.Destination = normalizeDestination(item.ToAccount)
		out.DestinationIsInternal = true
	case actiondomain.ActionTypeHotToColdMove:
		var item actiondomain.HotToColdMovePayload
		if err := json.Unmarshal(input.Payload, &item); err != nil {
			return signals{}, fmt.Errorf("decode hot to cold payload for risk evaluation: %w", err)
		}
		out.HasAmount = strings.TrimSpace(item.Amount) != ""
		out.Amount = parseAmount(item.Amount)
		out.HasDestination = strings.TrimSpace(item.ToWallet) != ""
		out.Destination = normalizeDestination(item.ToWallet)
		out.DestinationIsInternal = true
	default:
		return signals{}, fmt.Errorf("unsupported action type for risk evaluation: %s", input.ActionType)
	}
	return out, nil
}

func evaluateFactors(profile profile, resource actiondomain.ProtectedResource, signals signals, ctx Context) []actiondomain.RiskFactor {
	factors := make([]actiondomain.RiskFactor, 0, 10)

	amountAnomaly := evaluateAmountAnomaly(profile, resource, signals, ctx)
	velocitySpike := evaluateVelocitySpike(profile, signals, ctx)
	newDestination := evaluateNewDestination(profile, signals, ctx)
	offHours, businessHours := evaluateBusinessHours(profile, signals, ctx)
	recentIncident := evaluateRecentIncident(profile, signals)
	knownDestination := evaluateKnownDestination(profile, signals, ctx)
	verifiedActor := evaluateVerifiedActor(profile, signals)

	actorDeviationActive := velocitySpike.Active && (offHours.Active || newDestination.Active)
	actorDeviation := newFactor(
		factorActorDeviation,
		actiondomain.RiskFactorTypePro,
		actorDeviationActive,
		profile.Weights[factorActorDeviation],
		profile.Weights[factorActorDeviation],
		"composite actor deviation detected from velocity and contextual anomalies",
		evidenceQualityFromActive(actorDeviationActive, actiondomain.EvidenceQualityObserved, actiondomain.EvidenceQualityInferred),
	)

	withinBaselineActive := !amountAnomaly.Active &&
		!velocitySpike.Active &&
		!newDestination.Active &&
		!offHours.Active &&
		!actorDeviation.Active &&
		!recentIncident.Active
	withinBaseline := newFactor(
		factorWithinBaseline,
		actiondomain.RiskFactorTypeAnti,
		withinBaselineActive,
		profile.Weights[factorWithinBaseline],
		profile.Weights[factorWithinBaseline],
		"no pro-risk factor fired for this action",
		evidenceQualityFromActive(withinBaselineActive, actiondomain.EvidenceQualityObserved, actiondomain.EvidenceQualityInferred),
	)

	factors = append(factors,
		amountAnomaly,
		velocitySpike,
		newDestination,
		offHours,
		actorDeviation,
		recentIncident,
		knownDestination,
		withinBaseline,
		businessHours,
		verifiedActor,
	)
	return factors
}

func evaluateAmountAnomaly(profile profile, resource actiondomain.ProtectedResource, signals signals, ctx Context) actiondomain.RiskFactor {
	baseline, ok := ctx.ResourceBaselines[MetricAvgTxAmount]
	if !signals.HasAmount {
		return newFactor(factorAmountAnomaly, actiondomain.RiskFactorTypePro, false, profile.Weights[factorAmountAnomaly], 0, "amount signal missing; amount anomaly not evaluated", actiondomain.EvidenceQualityMissing)
	}
	if !ok || baseline.SampleSize == 0 {
		appliedWeight := 0.05
		if isCriticalResource(resource.Criticality) {
			appliedWeight = profile.Weights[factorAmountAnomaly]
		}
		return newFactor(
			factorAmountAnomaly,
			actiondomain.RiskFactorTypePro,
			true,
			profile.Weights[factorAmountAnomaly],
			appliedWeight,
			"baseline missing in cold start; reduced amount anomaly weight applied",
			actiondomain.EvidenceQualityMissing,
		)
	}

	confidence := baseline.Confidence()
	spread := math.Max(baseline.Stddev, math.Max(baseline.Avg*0.10, 1))
	threshold := baseline.Avg + (3/max(confidence, 0.1))*spread
	active := signals.Amount > threshold
	quality := actiondomain.EvidenceQualityObserved
	if baseline.IsStale(signals.Now) {
		quality = actiondomain.EvidenceQualityStale
	}
	return newFactor(
		factorAmountAnomaly,
		actiondomain.RiskFactorTypePro,
		active,
		profile.Weights[factorAmountAnomaly],
		profile.Weights[factorAmountAnomaly],
		fmt.Sprintf("amount %.2f compared against baseline threshold %.2f", signals.Amount, threshold),
		quality,
	)
}

func evaluateVelocitySpike(profile profile, signals signals, ctx Context) actiondomain.RiskFactor {
	baseline, ok := ctx.ActorBaselines[MetricActions30mCount]
	if !ok || baseline.SampleSize == 0 {
		return newFactor(factorVelocitySpike, actiondomain.RiskFactorTypePro, false, profile.Weights[factorVelocitySpike], 0, "30m actor velocity baseline missing", actiondomain.EvidenceQualityMissing)
	}
	threshold := math.Max(1, baseline.P95)
	active := float64(signals.RecentActorCount30) > threshold
	quality := actiondomain.EvidenceQualityObserved
	if baseline.IsStale(signals.Now) {
		quality = actiondomain.EvidenceQualityStale
	}
	return newFactor(
		factorVelocitySpike,
		actiondomain.RiskFactorTypePro,
		active,
		profile.Weights[factorVelocitySpike],
		profile.Weights[factorVelocitySpike],
		fmt.Sprintf("recent actor count in 30m is %d vs p95 %.2f", signals.RecentActorCount30, baseline.P95),
		quality,
	)
}

func evaluateNewDestination(profile profile, signals signals, ctx Context) actiondomain.RiskFactor {
	if !signals.HasDestination {
		return newFactor(factorNewDestination, actiondomain.RiskFactorTypePro, false, profile.Weights[factorNewDestination], 0, "no destination signal available; new destination factor inactive", actiondomain.EvidenceQualityInferred)
	}
	appliedWeight := profile.Weights[factorNewDestination]
	if signals.DestinationIsInternal {
		appliedWeight = roundTwo(appliedWeight * 0.33)
	}
	if ctx.KnownDestination == nil {
		return newFactor(factorNewDestination, actiondomain.RiskFactorTypePro, true, profile.Weights[factorNewDestination], appliedWeight, "destination history missing; destination treated as new", actiondomain.EvidenceQualityMissing)
	}
	active := ctx.KnownDestination.Confidence < newDestinationThreshold
	return newFactor(
		factorNewDestination,
		actiondomain.RiskFactorTypePro,
		active,
		profile.Weights[factorNewDestination],
		appliedWeight,
		fmt.Sprintf("destination confidence is %.2f", ctx.KnownDestination.Confidence),
		actiondomain.EvidenceQualityObserved,
	)
}

func evaluateBusinessHours(profile profile, signals signals, ctx Context) (actiondomain.RiskFactor, actiondomain.RiskFactor) {
	baseline, ok := ctx.ActorBaselines[MetricTypicalHours]
	if !ok || baseline.SampleSize == 0 {
		offHours := newFactor(factorOffHours, actiondomain.RiskFactorTypePro, false, profile.Weights[factorOffHours], 0, "typical hours baseline missing", actiondomain.EvidenceQualityMissing)
		businessHours := newFactor(factorBusinessHours, actiondomain.RiskFactorTypeAnti, false, profile.Weights[factorBusinessHours], 0, "typical hours baseline missing", actiondomain.EvidenceQualityMissing)
		return offHours, businessHours
	}
	hour := float64(signals.Now.UTC().Hour())
	spread := math.Max(2, baseline.Stddev*2)
	lower := math.Max(0, baseline.Avg-spread)
	upper := math.Min(23, baseline.Avg+spread)
	inHours := hour >= lower && hour <= upper
	quality := actiondomain.EvidenceQualityObserved
	if baseline.IsStale(signals.Now) {
		quality = actiondomain.EvidenceQualityStale
	}
	offHours := newFactor(
		factorOffHours,
		actiondomain.RiskFactorTypePro,
		!inHours,
		profile.Weights[factorOffHours],
		profile.Weights[factorOffHours],
		fmt.Sprintf("current hour %.0f vs typical window %.1f-%.1f", hour, lower, upper),
		quality,
	)
	businessHours := newFactor(
		factorBusinessHours,
		actiondomain.RiskFactorTypeAnti,
		inHours,
		profile.Weights[factorBusinessHours],
		profile.Weights[factorBusinessHours],
		fmt.Sprintf("current hour %.0f vs typical window %.1f-%.1f", hour, lower, upper),
		quality,
	)
	return offHours, businessHours
}

func evaluateRecentIncident(profile profile, signals signals) actiondomain.RiskFactor {
	active := signals.OpenIncidentCount > 0
	return newFactor(
		factorRecentIncident,
		actiondomain.RiskFactorTypePro,
		active,
		profile.Weights[factorRecentIncident],
		profile.Weights[factorRecentIncident],
		fmt.Sprintf("%d open incidents on the protected resource", signals.OpenIncidentCount),
		evidenceQualityFromActive(active, actiondomain.EvidenceQualityObserved, actiondomain.EvidenceQualityInferred),
	)
}

func evaluateKnownDestination(profile profile, signals signals, ctx Context) actiondomain.RiskFactor {
	if ctx.KnownDestination == nil || !signals.HasDestination {
		return newFactor(factorKnownDestination, actiondomain.RiskFactorTypeAnti, false, profile.Weights[factorKnownDestination], 0, "destination not known yet", actiondomain.EvidenceQualityMissing)
	}
	weight := profile.Weights[factorKnownDestination]
	if signals.DestinationIsInternal {
		weight = roundTwo(weight * 0.5)
	}
	active := ctx.KnownDestination.Confidence >= knownDestinationConfidence
	return newFactor(
		factorKnownDestination,
		actiondomain.RiskFactorTypeAnti,
		active,
		profile.Weights[factorKnownDestination],
		weight,
		fmt.Sprintf("destination confidence is %.2f with %d prior hits", ctx.KnownDestination.Confidence, ctx.KnownDestination.TxCount),
		actiondomain.EvidenceQualityObserved,
	)
}

func evaluateVerifiedActor(profile profile, signals signals) actiondomain.RiskFactor {
	return newFactor(
		factorVerifiedActor,
		actiondomain.RiskFactorTypeAnti,
		signals.VerifiedActor,
		profile.Weights[factorVerifiedActor],
		profile.Weights[factorVerifiedActor],
		"verified actor context present",
		evidenceQualityFromActive(signals.VerifiedActor, actiondomain.EvidenceQualityObserved, actiondomain.EvidenceQualityMissing),
	)
}

func newFactor(code string, factorType actiondomain.RiskFactorType, active bool, weight float64, appliedWeight float64, summary string, quality actiondomain.EvidenceQuality) actiondomain.RiskFactor {
	return actiondomain.RiskFactor{
		Code:            code,
		Type:            factorType,
		Active:          active,
		Weight:          roundTwo(weight),
		AppliedWeight:   activeWeight(active, appliedWeight),
		Summary:         summary,
		EvidenceQuality: quality,
	}
}

func activeInteractions(profiles []interactionProfile, factors []actiondomain.RiskFactor) []interactionProfile {
	activeSet := make(map[string]struct{}, len(factors))
	for _, factor := range factors {
		if factor.Active {
			activeSet[factor.Code] = struct{}{}
		}
	}
	out := make([]interactionProfile, 0, len(profiles))
	for _, profile := range profiles {
		all := true
		for _, factor := range profile.Factors {
			if _, ok := activeSet[factor]; !ok {
				all = false
				break
			}
		}
		if all {
			out = append(out, profile)
		}
	}
	return out
}

func toInteractions(profiles []interactionProfile) []actiondomain.RiskInteraction {
	items := make([]actiondomain.RiskInteraction, 0, len(profiles))
	for _, profile := range profiles {
		items = append(items, actiondomain.RiskInteraction{
			Factors:    append([]string(nil), profile.Factors...),
			Multiplier: profile.Multiplier,
			Summary:    profile.Summary,
		})
	}
	return items
}

func accumulatePressures(factors []actiondomain.RiskFactor) (float64, float64) {
	var riskPressure float64
	var safetyPressure float64
	for _, factor := range factors {
		if !factor.Active {
			continue
		}
		switch factor.Type {
		case actiondomain.RiskFactorTypeAnti:
			safetyPressure += factor.AppliedWeight
		default:
			riskPressure += factor.AppliedWeight
		}
	}
	return roundTwo(riskPressure), roundTwo(safetyPressure)
}

func capRiskMultiplier(items []interactionProfile) float64 {
	multiplier := combinedMultiplier(items)
	if multiplier > 3.0 {
		return 3.0
	}
	return multiplier
}

func combinedMultiplier(items []interactionProfile) float64 {
	if len(items) == 0 {
		return 1
	}
	out := 1.0
	for _, item := range items {
		out *= item.Multiplier
	}
	return roundTwo(out)
}

func classifyDecision(bands [4]float64, score float64) actiondomain.RiskDecision {
	switch {
	case score < bands[0]:
		return actiondomain.RiskDecisionAllow
	case score < bands[1]:
		return actiondomain.RiskDecisionEnhancedLog
	case score < bands[2]:
		return actiondomain.RiskDecisionAdditionalAuth
	case score < bands[3]:
		return actiondomain.RiskDecisionRequireApproval
	default:
		return actiondomain.RiskDecisionDeny
	}
}

func applyHysteresis(current actiondomain.RiskDecision, previous *actiondomain.RiskDecision, bands [4]float64, score float64) actiondomain.RiskDecision {
	if previous == nil || *previous == "" || *previous == current {
		return current
	}
	order := []actiondomain.RiskDecision{
		actiondomain.RiskDecisionAllow,
		actiondomain.RiskDecisionEnhancedLog,
		actiondomain.RiskDecisionAdditionalAuth,
		actiondomain.RiskDecisionRequireApproval,
		actiondomain.RiskDecisionDeny,
	}
	currentIdx := slices.Index(order, current)
	previousIdx := slices.Index(order, *previous)
	if currentIdx == -1 || previousIdx == -1 || abs(previousIdx-currentIdx) != 1 {
		return current
	}
	thresholdIdx := maxInt(currentIdx, previousIdx) - 1
	if thresholdIdx < 0 || thresholdIdx >= len(bands) {
		return current
	}
	threshold := bands[thresholdIdx]
	if math.Abs(score-threshold) <= hysteresisWindow {
		return *previous
	}
	return current
}

func classifyLevel(score int) actiondomain.RiskLevel {
	switch {
	case score < 20:
		return actiondomain.RiskLevelLow
	case score < 40:
		return actiondomain.RiskLevelMedium
	case score < 70:
		return actiondomain.RiskLevelHigh
	default:
		return actiondomain.RiskLevelCritical
	}
}

func buildSummary(profile profile, recommended actiondomain.RiskDecision, factors []actiondomain.RiskFactor) string {
	active := make([]string, 0, len(factors))
	for _, factor := range factors {
		if factor.Active {
			active = append(active, factor.Code)
		}
	}
	if len(active) == 0 {
		return fmt.Sprintf("%s v%d recommends %s with no active risk factors", profile.Name, profile.Version, recommended)
	}
	return fmt.Sprintf("%s v%d recommends %s from %s", profile.Name, profile.Version, recommended, strings.Join(active, ", "))
}

func actorVerified(actor actiondomain.ActorRef, metadata map[string]any, ctx Context) bool {
	if ctx.VerifiedActor {
		return true
	}
	if boolMetadata(metadata, "verified_actor") || boolMetadata(metadata, "ip_known") {
		return true
	}
	return actor.Type == actiondomain.ActorTypeSystem
}

func boolMetadata(metadata map[string]any, key string) bool {
	if len(metadata) == 0 {
		return false
	}
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

func normalizeDestination(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func parseAmount(raw string) float64 {
	value, err := strconvParseFloat(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return value
}

func strconvParseFloat(value string) (float64, error) {
	return strconv.ParseFloat(value, 64)
}

func activeWeight(active bool, weight float64) float64 {
	if !active {
		return 0
	}
	return roundTwo(weight)
}

func evidenceQualityFromActive(active bool, activeQuality, inactiveQuality actiondomain.EvidenceQuality) actiondomain.EvidenceQuality {
	if active {
		return activeQuality
	}
	return inactiveQuality
}

func isCriticalResource(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "critical")
}

func clampZeroToOne(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func roundTwo(value float64) float64 {
	return math.Round(value*100) / 100
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
