package foundation

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/foundation-go/foundation/http"
)

func TestGatewayOptionsValidate(t *testing.T) {
	t.Run("authentication without a details middleware is refused", func(t *testing.T) {
		opts := NewGatewayOptions()
		opts.WithAuthentication = true

		err := opts.Validate()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "AuthenticationDetailsMiddleware")
	})

	t.Run("authentication with a details middleware is accepted", func(t *testing.T) {
		opts := NewGatewayOptions()
		opts.WithAuthentication = true
		opts.AuthenticationDetailsMiddleware = func(h http.Handler) http.Handler { return h }

		assert.NoError(t, opts.Validate())
	})

	t.Run("no authentication needs no details middleware", func(t *testing.T) {
		assert.NoError(t, NewGatewayOptions().Validate())
	})

	// A gateway behind an authenticating proxy has nothing for a details
	// middleware to do — that deployment is what the option exists for, and
	// refusing to start it left no supported configuration at all.
	t.Run("a trusted proxy replaces the details middleware", func(t *testing.T) {
		opts := NewGatewayOptions()
		opts.WithAuthentication = true
		opts.TrustInboundAuthenticationHeaders = true

		assert.NoError(t, opts.Validate())
	})

	t.Run("the error points at every way out", func(t *testing.T) {
		opts := NewGatewayOptions()
		opts.WithAuthentication = true

		err := opts.Validate()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "AuthenticationDetailsMiddleware")
		assert.Contains(t, err.Error(), "TrustInboundAuthenticationHeaders")
	})
}

func newTestService() *Service {
	logger := logrus.New()
	logger.SetOutput(newDiscardWriter())

	return &Service{
		Name:   "test",
		Config: NewConfig(),
		Logger: logrus.NewEntry(logger),
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func newDiscardWriter() discardWriter { return discardWriter{} }

// The identity headers must be gone by the time the mux sees the request, no
// matter how the rest of the chain is configured.
func TestApplyMiddlewareStripsInboundIdentityHeaders(t *testing.T) {
	svc := newTestService()

	var seen http.Header
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	})

	handler := svc.applyMiddleware(mux, NewGatewayOptions())

	req := httptest.NewRequest(http.MethodGet, "/chats", nil)
	for _, header := range fhttp.AuthenticationHeaders {
		req.Header.Set(header, "spoofed")
	}

	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.NotNil(t, seen)
	for _, header := range fhttp.AuthenticationHeaders {
		assert.Empty(t, seen.Get(header), "%s should have been stripped", header)
	}
}

func TestApplyMiddlewareKeepsInboundHeadersWhenTrusted(t *testing.T) {
	svc := newTestService()

	var seen http.Header
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	})

	opts := NewGatewayOptions()
	opts.TrustInboundAuthenticationHeaders = true

	handler := svc.applyMiddleware(mux, opts)

	req := httptest.NewRequest(http.MethodGet, "/chats", nil)
	req.Header.Set(fhttp.HeaderXUserID, "from-trusted-proxy")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.NotNil(t, seen)
	assert.Equal(t, "from-trusted-proxy", seen.Get(fhttp.HeaderXUserID))
}
