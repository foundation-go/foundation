package foundation

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unhealthyComponent fails its health check.
type unhealthyComponent struct {
	fakeComponent
	reason error
}

func (c *unhealthyComponent) Health() error { return c.reason }

func (c *unhealthyComponent) HealthContext(context.Context) error { return c.reason }

// hangingComponent never returns from its health check and does not implement
// HealthCheckerContext, so it can only be abandoned.
type hangingComponent struct {
	fakeComponent
	release chan struct{}
}

func (c *hangingComponent) Health() error {
	<-c.release

	return nil
}

// panickingComponent panics from its health check, as an unstarted component
// with a nil dependency would.
type panickingComponent struct {
	fakeComponent
}

func (c *panickingComponent) Health() error { panic("nil dependency") }

func newHealthTestService(components ...Component) *Service {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	return &Service{
		Name:       "test",
		Config:     &Config{HealthCheckTimeout: 200 * time.Millisecond},
		Logger:     logrus.NewEntry(logger),
		Components: components,
	}
}

func probe(t *testing.T, handler http.HandlerFunc) (int, healthStatus) {
	t.Helper()

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	var status healthStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &status))

	return rec.Code, status
}

// Liveness must not depend on anything external: a failing liveness probe
// restarts the pod, and restarting a service because its database blipped only
// widens the outage.
func TestLivenessIgnoresComponents(t *testing.T) {
	svc := newHealthTestService(&unhealthyComponent{
		fakeComponent: fakeComponent{name: "postgresql"},
		reason:        errors.New("connection refused"),
	})

	code, status := probe(t, svc.livenessHandler)

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", status.Status)
}

func TestReadinessWithHealthyComponents(t *testing.T) {
	svc := newHealthTestService(&fakeComponent{name: "postgresql"}, &fakeComponent{name: "redis"})

	code, status := probe(t, svc.readinessHandler)

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", status.Status)
	assert.Empty(t, status.Components)
}

func TestReadinessWithNoComponents(t *testing.T) {
	code, status := probe(t, newHealthTestService().readinessHandler)

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", status.Status)
}

// Every unhealthy component is reported, not just the first: the handler used
// to bail out on the first failure with an empty body.
func TestReadinessReportsEveryUnhealthyComponent(t *testing.T) {
	svc := newHealthTestService(
		&unhealthyComponent{fakeComponent: fakeComponent{name: "postgresql"}, reason: errors.New("connection refused")},
		&fakeComponent{name: "redis"},
		&unhealthyComponent{fakeComponent: fakeComponent{name: "kafka-consumer"}, reason: errors.New("no brokers")},
	)

	code, status := probe(t, svc.readinessHandler)

	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "unavailable", status.Status)
	assert.Equal(t, map[string]string{
		"postgresql":     "connection refused",
		"kafka-consumer": "no brokers",
	}, status.Components)
}

// A component that hangs must not hang the probe with it.
func TestReadinessBoundsAHangingComponent(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	svc := newHealthTestService(&hangingComponent{
		fakeComponent: fakeComponent{name: "stuck"},
		release:       release,
	})
	svc.Config.HealthCheckTimeout = 20 * time.Millisecond

	done := make(chan struct{})
	go func() {
		defer close(done)

		code, status := probe(t, svc.readinessHandler)

		assert.Equal(t, http.StatusServiceUnavailable, code)
		assert.Contains(t, status.Components["stuck"], "did not finish in time")
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the readiness probe blocked on a hanging component")
	}
}

// A component that has not been started yet — the metrics server is registered
// before Redis and the jobs enqueuer, so probes do arrive in that window — used
// to take the handler down with it.
func TestReadinessContainsAPanickingComponent(t *testing.T) {
	svc := newHealthTestService(&panickingComponent{fakeComponent: fakeComponent{name: "jobs-enqueuer"}})

	var (
		code   int
		status healthStatus
	)

	require.NotPanics(t, func() { code, status = probe(t, svc.readinessHandler) })

	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Contains(t, status.Components["jobs-enqueuer"], "panic in health check")
}

// While the service is shutting down it must stop advertising itself as ready,
// even though every component is still healthy.
func TestReadinessFailsWhileDraining(t *testing.T) {
	svc := newHealthTestService(&fakeComponent{name: "postgresql"})
	svc.draining.Store(true)

	code, status := probe(t, svc.readinessHandler)

	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "draining", status.Status)
	assert.True(t, svc.IsDraining())
}

// `/health` predates the split and keeps readiness semantics.
func TestHealthHandlerKeepsReadinessSemantics(t *testing.T) {
	healthy := newHealthTestService(&fakeComponent{name: "redis"})
	code, _ := probe(t, healthy.healthHandler)
	assert.Equal(t, http.StatusOK, code)

	unhealthy := newHealthTestService(&unhealthyComponent{
		fakeComponent: fakeComponent{name: "redis"},
		reason:        errors.New("connection refused"),
	})
	code, _ = probe(t, unhealthy.healthHandler)
	assert.Equal(t, http.StatusServiceUnavailable, code)
}

func TestHealthCheckTimeoutFallsBackToDefault(t *testing.T) {
	assert.Equal(t, DefaultHealthCheckTimeout, (&Service{}).healthCheckTimeout())
	assert.Equal(t, DefaultHealthCheckTimeout, (&Service{Config: &Config{}}).healthCheckTimeout())
	assert.Equal(t, time.Second, (&Service{Config: &Config{HealthCheckTimeout: time.Second}}).healthCheckTimeout())
}

func TestDrainDelayDefaultsToZero(t *testing.T) {
	assert.Zero(t, (&Service{}).drainDelay())
	assert.Equal(t, 5*time.Second, (&Service{Config: &Config{DrainDelay: 5 * time.Second}}).drainDelay())
}

func TestMetricsServerRegistersProbeEndpoints(t *testing.T) {
	svc := newHealthTestService(&fakeComponent{name: "redis"})

	component := NewMetricsServerComponent(
		WithMetricsServerHealthHandler(svc.healthHandler),
		WithMetricsServerLivenessHandler(svc.livenessHandler),
		WithMetricsServerReadinessHandler(svc.readinessHandler),
	)

	for _, path := range []string{"/health", "/live", "/ready", "/metrics"} {
		rec := httptest.NewRecorder()
		component.server.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		assert.Equal(t, http.StatusOK, rec.Code, "%s should be served", path)
	}
}

// The mux panics on a nil handler; a component built without probe handlers
// must still serve metrics.
func TestMetricsServerToleratesMissingProbeHandlers(t *testing.T) {
	var component *MetricsServerComponent

	require.NotPanics(t, func() { component = NewMetricsServerComponent() })

	rec := httptest.NewRecorder()
	component.server.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	component.server.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMetricsServerHasAReadHeaderTimeout(t *testing.T) {
	assert.Positive(t, NewMetricsServerComponent().server.ReadHeaderTimeout)
}

// A shared deadline across a sequential loop meant the first slow component
// consumed it and every component after that was reported down with "context
// deadline exceeded" — one unreachable database accused Redis and Kafka too.
func TestReadinessIsolatesASlowComponent(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	svc := newHealthTestService(
		&hangingComponent{fakeComponent: fakeComponent{name: "slow"}, release: release},
		&fakeComponent{name: "redis"},
		&fakeComponent{name: "kafka-consumer"},
	)
	svc.Config.HealthCheckTimeout = 30 * time.Millisecond

	code, status := probe(t, svc.readinessHandler)

	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Contains(t, status.Components, "slow")
	assert.NotContains(t, status.Components, "redis", "a healthy component must not be blamed for a slow one")
	assert.NotContains(t, status.Components, "kafka-consumer")
}
