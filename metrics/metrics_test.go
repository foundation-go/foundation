package metrics

import (
	"errors"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetComponentUp(t *testing.T) {
	SetComponentUp("postgresql", true)
	assert.Equal(t, 1.0, testutil.ToFloat64(ComponentUp.WithLabelValues("postgresql")))

	SetComponentUp("postgresql", false)
	assert.Equal(t, 0.0, testutil.ToFloat64(ComponentUp.WithLabelValues("postgresql")))
}

func TestSetDraining(t *testing.T) {
	SetDraining(true)
	assert.Equal(t, 1.0, testutil.ToFloat64(Draining))

	SetDraining(false)
	assert.Equal(t, 0.0, testutil.ToFloat64(Draining))
}

func TestResultOf(t *testing.T) {
	assert.Equal(t, ResultSuccess, ResultOf(nil))
	assert.Equal(t, ResultError, ResultOf(errors.New("boom")))
}

// Everything this package defines has to end up on the registry the `/metrics`
// endpoint serves, and carry the shared namespace.
func TestMetricsAreRegisteredUnderTheNamespace(t *testing.T) {
	// Touch the label-bearing metrics so that they have at least one child to
	// gather.
	HTTPRequests.WithLabelValues("GET", "/v1/chats", "200").Inc()
	HTTPRequestDuration.WithLabelValues("GET", "/v1/chats").Observe(0.01)
	GRPCRequests.WithLabelValues("/chats.Chats/Get", "OK").Inc()
	EventsProcessed.WithLabelValues("chats", "chats.MessageSent", ResultSuccess).Inc()
	JobsProcessed.WithLabelValues("send_email", ResultSuccess).Inc()
	Panics.WithLabelValues("http").Inc()
	OutboxPublishedEvents.Inc()
	SetComponentUp("redis", true)

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	found := map[string]bool{}
	for _, family := range families {
		if strings.HasPrefix(family.GetName(), Namespace+"_") {
			found[family.GetName()] = true
		}
	}

	for _, name := range []string{
		"foundation_http_requests_total",
		"foundation_http_request_duration_seconds",
		"foundation_grpc_requests_total",
		"foundation_events_processed_total",
		"foundation_outbox_published_events_total",
		"foundation_outbox_pending_events",
		"foundation_jobs_processed_total",
		"foundation_service_component_up",
		"foundation_service_panics_total",
	} {
		assert.True(t, found[name], "%s should be exposed on /metrics", name)
	}
}
