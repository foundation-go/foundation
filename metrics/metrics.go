// Package metrics holds the Prometheus metrics Foundation records on behalf of
// a service.
//
// Before this package the `/metrics` endpoint served only the default Go and
// process collectors: not a single metric described what the service actually
// did. There was no way to see a request rate, an error rate, a latency
// distribution, or how far behind a consumer had fallen.
//
// Every label here is deliberately low-cardinality. HTTP requests are labelled
// by the matched route pattern rather than the request path, so `/v1/chats/1`
// and `/v1/chats/2` share a series instead of creating one each.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Namespace prefixes every metric this package defines.
const Namespace = "foundation"

// Result values used by the `result` label.
const (
	ResultSuccess = "success"
	ResultError   = "error"
	ResultSkipped = "skipped"
)

// HTTP metrics, recorded by the gateway and the HTTP server mode.
var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Number of HTTP requests handled, by method, matched route and response status.",
	}, []string{"method", "route", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "Time spent handling an HTTP request.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route"})

	HTTPRequestsInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "http",
		Name:      "requests_in_flight",
		Help:      "Number of HTTP requests currently being handled.",
	})
)

// gRPC server metrics.
var (
	GRPCRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "grpc",
		Name:      "requests_total",
		Help:      "Number of gRPC calls handled, by method and resulting status code.",
	}, []string{"method", "code"})

	GRPCRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: "grpc",
		Name:      "request_duration_seconds",
		Help:      "Time spent handling a gRPC call.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method"})
)

// Events worker metrics.
var (
	EventsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "events",
		Name:      "processed_total",
		Help:      "Number of events read from Kafka, by topic, event type and outcome.",
	}, []string{"topic", "event", "result"})

	EventProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: "events",
		Name:      "processing_duration_seconds",
		Help:      "Time spent handling a single event, handlers and transaction included.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"event"})

	// EventLag measures how long an event waited between being produced and
	// being handled. It is the signal that tells a growing backlog apart from a
	// slow handler.
	EventLag = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: "events",
		Name:      "lag_seconds",
		Help:      "Delay between an event being produced and being handled.",
		Buckets:   prometheus.ExponentialBuckets(0.05, 3, 10),
	}, []string{"topic"})
)

// Outbox metrics.
var (
	OutboxPendingEvents = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "outbox",
		Name:      "pending_events",
		Help:      "Events waiting in the outbox at the end of the last courier run.",
	})

	OutboxOldestEventAge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "outbox",
		Name:      "oldest_event_age_seconds",
		Help:      "Age of the oldest event still in the outbox. The metric to alert on.",
	})

	OutboxPublishedEvents = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "outbox",
		Name:      "published_events_total",
		Help:      "Events published to Kafka by the outbox courier.",
	})

	OutboxBatchDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: "outbox",
		Name:      "batch_duration_seconds",
		Help:      "Time spent publishing and deleting one batch of outbox events.",
		Buckets:   prometheus.DefBuckets,
	})
)

// Database metrics, recorded by the pgx query tracer.
var (
	DatabaseQueries = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "database",
		Name:      "queries_total",
		Help:      "Number of SQL statements executed, by operation and outcome.",
	}, []string{"operation", "result"})

	DatabaseQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: "database",
		Name:      "query_duration_seconds",
		Help:      "Time spent executing a SQL statement.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"operation"})
)

// Connection pool descriptors.
//
// These are read from pgxpool at scrape time rather than being pushed, so they
// are exposed through a Collector. Pool exhaustion is a common and confusing
// production failure: the service looks idle, every request is slow, and
// nothing in the logs explains why.
var (
	DatabasePoolTotalConns = prometheus.NewDesc(
		Namespace+"_database_pool_total_connections",
		"Connections currently held by the pool, idle and in use.",
		nil, nil,
	)
	DatabasePoolIdleConns = prometheus.NewDesc(
		Namespace+"_database_pool_idle_connections",
		"Connections currently idle in the pool.",
		nil, nil,
	)
	DatabasePoolAcquiredConns = prometheus.NewDesc(
		Namespace+"_database_pool_acquired_connections",
		"Connections currently checked out of the pool.",
		nil, nil,
	)
	DatabasePoolMaxConns = prometheus.NewDesc(
		Namespace+"_database_pool_max_connections",
		"Maximum number of connections the pool may open.",
		nil, nil,
	)
)

// Background jobs metrics.
var (
	JobsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: "jobs",
		Name:      "processed_total",
		Help:      "Number of background jobs run, by name and outcome.",
	}, []string{"job", "result"})

	JobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: "jobs",
		Name:      "duration_seconds",
		Help:      "Time spent running a background job.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"job"})
)

// Service and component metrics.
var (
	// ComponentUp is 1 when a component's last health check succeeded, 0 when
	// it failed. It turns an outage into a graph instead of a wall of log
	// lines, and replaces the Sentry event the probe used to send on every
	// single failed check.
	ComponentUp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "service",
		Name:      "component_up",
		Help:      "Whether a component reported itself healthy at the last readiness check.",
	}, []string{"component"})

	// Draining is 1 once the service has begun shutting down.
	Draining = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "service",
		Name:      "draining",
		Help:      "Whether the service is shutting down and refusing readiness.",
	})

	// Info carries build metadata as labels on a constant 1, the conventional
	// way to expose a version to Prometheus.
	Info = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "service",
		Name:      "info",
		Help:      "Build and runtime information about the service.",
	}, []string{"service", "mode", "environment", "version"})
)

// Panics recovered anywhere in the framework.
var Panics = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: Namespace,
	Subsystem: "service",
	Name:      "panics_total",
	Help:      "Panics recovered by Foundation, by where they happened.",
}, []string{"source"})

// SetComponentUp records the outcome of a component's health check.
func SetComponentUp(component string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}

	ComponentUp.WithLabelValues(component).Set(value)
}

// SetDraining records whether the service is shutting down.
func SetDraining(draining bool) {
	value := 0.0
	if draining {
		value = 1.0
	}

	Draining.Set(value)
}

// ResultOf maps an error to the value of a `result` label.
func ResultOf(err error) string {
	if err != nil {
		return ResultError
	}

	return ResultSuccess
}
