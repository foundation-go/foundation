package foundation

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every Foundation HTTP server was built without a single timeout, which is
// gosec G112: a handful of connections dribbling headers keeps the server busy
// indefinitely.
func TestNewHTTPServerAppliesTimeouts(t *testing.T) {
	svc := newTestService()

	server := svc.newHTTPServer(8080, http.NotFoundHandler())

	assert.Equal(t, ":8080", server.Addr)
	assert.Positive(t, server.ReadHeaderTimeout, "ReadHeaderTimeout is the Slowloris fix")
	assert.Positive(t, server.IdleTimeout)
}

func TestNewHTTPServerHonoursConfiguration(t *testing.T) {
	svc := newTestService()
	svc.Config.HTTP = &HTTPConfig{
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       4 * time.Second,
	}

	server := svc.newHTTPServer(1234, http.NotFoundHandler())

	assert.Equal(t, time.Second, server.ReadHeaderTimeout)
	assert.Equal(t, 2*time.Second, server.ReadTimeout)
	assert.Equal(t, 3*time.Second, server.WriteTimeout)
	assert.Equal(t, 4*time.Second, server.IdleTimeout)
}

// A service built without a Config must still get the protective defaults.
func TestHTTPConfigFallsBackToDefaults(t *testing.T) {
	cfg := (&Service{}).httpConfig()

	assert.Equal(t, DefaultHTTPReadHeaderTimeout, cfg.ReadHeaderTimeout)
	assert.Equal(t, DefaultHTTPIdleTimeout, cfg.IdleTimeout)
	assert.Equal(t, DefaultMaxRequestBodySize, cfg.MaxRequestBodySize)
}

// grpc-gateway unmarshals the whole request body into a protobuf message, so an
// unbounded body is an unbounded allocation.
func TestNewHTTPServerCapsTheRequestBody(t *testing.T) {
	svc := newTestService()
	svc.Config.HTTP = &HTTPConfig{MaxRequestBodySize: 16}

	var readErr error

	server := svc.newHTTPServer(0, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 1024)))
	server.Handler.ServeHTTP(httptest.NewRecorder(), req)

	require.Error(t, readErr, "reading past the cap must fail")
	assert.Contains(t, readErr.Error(), "too large")
}

func TestNewHTTPServerLeavesTheBodyUncappedWhenDisabled(t *testing.T) {
	svc := newTestService()
	svc.Config.HTTP = &HTTPConfig{MaxRequestBodySize: 0}

	var read int

	server := svc.newHTTPServer(0, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		read = len(body)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 1024)))
	server.Handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, 1024, read)
}

func TestNewHTTPConfigReadsTheEnvironment(t *testing.T) {
	t.Setenv("HTTP_READ_HEADER_TIMEOUT", "3s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "45s")
	t.Setenv("HTTP_MAX_REQUEST_BODY_SIZE", "1024")

	cfg := NewHTTPConfig()

	assert.Equal(t, 3*time.Second, cfg.ReadHeaderTimeout)
	assert.Equal(t, 45*time.Second, cfg.WriteTimeout)
	assert.Equal(t, int64(1024), cfg.MaxRequestBodySize)
}

func TestShutdownHTTPServerIsBounded(t *testing.T) {
	svc := newTestService()
	svc.Config.ShutdownTimeout = 20 * time.Millisecond

	server := svc.newHTTPServer(0, http.NotFoundHandler())

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = svc.shutdownHTTPServer(server)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdownHTTPServer did not respect the shutdown budget")
	}
}
