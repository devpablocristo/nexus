package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	EventsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nexus_operators",
		Name:      "events_processed_total",
		Help:      "Total events processed by each worker.",
	}, []string{"worker", "status"})

	EventDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "nexus_operators",
		Name:      "event_processing_duration_seconds",
		Help:      "Duration of event handler processing.",
		Buckets:   []float64{.001, .005, .01, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"worker"})

	ConsumerOffset = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "nexus_operators",
		Name:      "consumer_offset",
		Help:      "Last acknowledged sequence per consumer group.",
	}, []string{"consumer_group"})

	CoreRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nexus_operators",
		Name:      "core_requests_total",
		Help:      "HTTP requests to nexus-core.",
	}, []string{"method", "status"})

	DeadLetterEvents = promauto.NewCounter(prometheus.CounterOpts{
		Name: "nexus_dead_letter_events_total",
		Help: "Total number of permanently failed events written to dead-letter logs.",
	})
)

func Handler() http.Handler {
	return promhttp.Handler()
}
