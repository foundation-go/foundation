package gateway

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/foundation-go/foundation/http"
)

func mockAuthHandler(token string) (*AuthenticationResult, error) {
	switch token {
	case "valid_token":
		return &AuthenticationResult{IsAuthenticated: true, UserID: "user_id"}, nil
	case "invalid_token":
		return &AuthenticationResult{IsAuthenticated: false, UserID: ""}, nil
	default:
		return nil, errors.New("server error")
	}
}

// captureHeaders returns a handler that records the identity headers it sees.
func captureHeaders(dst *http.Header) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*dst = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	})
}

func TestWithAuthenticationDetails(t *testing.T) {
	tests := []struct {
		name              string
		token             string
		wantAuthenticated string
		wantUserID        string
	}{
		{"valid token", "valid_token", "true", "user_id"},
		{"invalid token", "invalid_token", "false", ""},
		// The authentication backend erroring out must fail closed rather than
		// dereference the nil result it returned.
		{"backend error", "unknown_token", "false", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen http.Header

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(fhttp.HeaderAuthorization, tt.token)
			rec := httptest.NewRecorder()

			WithAuthenticationDetails(captureHeaders(&seen), mockAuthHandler).ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.wantAuthenticated, seen.Get(fhttp.HeaderXAuthenticated))
			assert.Equal(t, tt.wantUserID, seen.Get(fhttp.HeaderXUserID))
		})
	}
}

func TestWithAuthenticationDetailsStripsBearerPrefix(t *testing.T) {
	var seen http.Header

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(fhttp.HeaderAuthorization, "Bearer valid_token")
	rec := httptest.NewRecorder()

	WithAuthenticationDetails(captureHeaders(&seen), mockAuthHandler).ServeHTTP(rec, req)

	assert.Equal(t, "true", seen.Get(fhttp.HeaderXAuthenticated))
	assert.Equal(t, "user_id", seen.Get(fhttp.HeaderXUserID))
}

// WithAuthentication enforces the decision recorded in `X-Authenticated`. It
// trusts that header, which is only safe because
// StripClientAuthenticationHeaders runs first — see
// TestSpoofedIdentityHeadersAreRejected for the property that actually matters.
func TestWithAuthentication(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("authenticated request passes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(fhttp.HeaderXAuthenticated, "true")
		rec := httptest.NewRecorder()

		WithAuthentication(nil)(mockHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		WithAuthentication(nil)(mockHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("excepted path passes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/signup", nil)
		rec := httptest.NewRecorder()

		WithAuthentication([]string{"/signup"})(mockHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestStripClientAuthenticationHeaders(t *testing.T) {
	var seen http.Header

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, header := range fhttp.AuthenticationHeaders {
		req.Header.Set(header, "spoofed")
	}
	// Non-identity Foundation headers are legitimate input and must survive.
	req.Header.Set(fhttp.HeaderXRequestID, "req-1")
	req.Header.Set(fhttp.HeaderAuthorization, "Bearer token")

	StripClientAuthenticationHeaders(captureHeaders(&seen)).ServeHTTP(httptest.NewRecorder(), req)

	for _, header := range fhttp.AuthenticationHeaders {
		assert.Empty(t, seen.Get(header), "%s should have been stripped", header)
	}
	assert.Equal(t, "req-1", seen.Get(fhttp.HeaderXRequestID))
	assert.Equal(t, "Bearer token", seen.Get(fhttp.HeaderAuthorization))
}

// The vulnerability: `X-Authenticated: true` supplied by the client used to be
// enough to pass WithAuthentication and to reach downstream services carrying
// an arbitrary `X-User-Id`.
func TestSpoofedIdentityHeadersAreRejected(t *testing.T) {
	var seen http.Header

	// The full gateway chain: strip → resolve identity → enforce.
	chain := StripClientAuthenticationHeaders(
		WithAuthenticationDetails(
			WithAuthentication(nil)(captureHeaders(&seen)),
			mockAuthHandler,
		),
	)

	t.Run("spoofed headers without a token are rejected", func(t *testing.T) {
		seen = nil

		req := httptest.NewRequest(http.MethodGet, "/private", nil)
		req.Header.Set(fhttp.HeaderXAuthenticated, "true")
		req.Header.Set(fhttp.HeaderXUserID, "00000000-0000-0000-0000-000000000001")
		req.Header.Set(fhttp.HeaderXScope, "admin")
		rec := httptest.NewRecorder()

		chain.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Nil(t, seen, "the request must not reach the application")
	})

	t.Run("spoofed identity is replaced by the authenticated one", func(t *testing.T) {
		seen = nil

		req := httptest.NewRequest(http.MethodGet, "/private", nil)
		req.Header.Set(fhttp.HeaderAuthorization, "Bearer valid_token")
		req.Header.Set(fhttp.HeaderXUserID, "00000000-0000-0000-0000-000000000001")
		req.Header.Set(fhttp.HeaderXScope, "admin")
		rec := httptest.NewRecorder()

		chain.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.NotNil(t, seen)
		assert.Equal(t, "user_id", seen.Get(fhttp.HeaderXUserID))
		assert.Empty(t, seen.Get(fhttp.HeaderXScope), "the spoofed scope must not survive")
	})
}
