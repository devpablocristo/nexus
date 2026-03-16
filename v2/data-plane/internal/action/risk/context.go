package risk

import (
	"math"
	"time"

	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

type ScopeType string

const (
	ScopeTypeResource ScopeType = "resource"
	ScopeTypeActor    ScopeType = "actor"
)

type Metric string

const (
	MetricDailyTxCount         Metric = "daily_tx_count"
	MetricDailyVolume          Metric = "daily_volume"
	MetricAvgTxAmount          Metric = "avg_tx_amount"
	MetricUniqueDestinations   Metric = "unique_destinations_daily"
	MetricDailyActionCount     Metric = "daily_action_count"
	MetricActions30mCount      Metric = "actions_30m_count"
	MetricTypicalHours         Metric = "typical_hours"
	knownDestinationConfidence        = 0.70
	newDestinationThreshold           = 0.35
	defaultStaleAfter                 = 2 * time.Hour
	hysteresisWindow                  = 0.03
)

type Baseline struct {
	ScopeType  ScopeType
	ScopeID    string
	Metric     Metric
	Avg        float64
	Stddev     float64
	P95        float64
	SampleSize int
	WindowDays int
	ComputedAt time.Time
}

func (b Baseline) Confidence() float64 {
	if b.SampleSize <= 0 {
		return 0
	}
	return 1 - math.Exp(-float64(b.SampleSize)/10.0)
}

func (b Baseline) IsStale(now time.Time) bool {
	if b.ComputedAt.IsZero() {
		return true
	}
	return now.Sub(b.ComputedAt) > defaultStaleAfter
}

type KnownDestination struct {
	ResourceID   string
	Destination  string
	FirstSeen    time.Time
	LastSeen     time.Time
	TxCount      int
	IsInternal   bool
	Confidence   float64
	ObservedDays float64
}

type Context struct {
	Now                time.Time
	ResourceBaselines  map[Metric]Baseline
	ActorBaselines     map[Metric]Baseline
	KnownDestination   *KnownDestination
	RecentActorCount30 int
	OpenIncidentCount  int
	VerifiedActor      bool
	PreviousDecision   *actiondomain.RiskDecision
}
