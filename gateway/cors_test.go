package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/foundation-go/foundation/http"
)

func corsHandler(options *CORSOptions) http.Handler {
	return WithCORSEnabled(options)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

func preflight(t *testing.T, options *CORSOptions, origin string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodOptions, "/v1/chats", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	rec := httptest.NewRecorder()
	corsHandler(options).ServeHTTP(rec, req)

	return rec
}

// The default was POST alone, so a browser refused every GET, PUT, PATCH and
// DELETE the API exposed unless the service listed them itself.
func TestCORSAllowsTheUsualRESTMethodsByDefault(t *testing.T) {
	rec := preflight(t, NewCORSOptions(), "https://example.com")

	allowed := rec.Header().Get(fhttp.HeaderAccessControlAllowMethods)

	for _, method := range []string{
		http.MethodGet, http.MethodHead, http.MethodPost,
		http.MethodPut, http.MethodPatch, http.MethodDelete,
	} {
		assert.Contains(t, allowed, method)
	}
}

func TestCORSAllowsAnyOriginByDefault(t *testing.T) {
	rec := preflight(t, NewCORSOptions(), "https://anywhere.example")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, AnyOrigin, rec.Header().Get(fhttp.HeaderAccessControlAllowOrigin))
}

// A single allowed origin could be configured, but never a list — which is what
// nearly every deployment actually has.
func TestCORSAllowsAListOfOrigins(t *testing.T) {
	options := NewCORSOptions()
	options.AllowedOrigins = []string{"https://app.example", "https://admin.example"}

	for _, origin := range options.AllowedOrigins {
		rec := preflight(t, options, origin)

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, origin, rec.Header().Get(fhttp.HeaderAccessControlAllowOrigin))
		assert.Contains(t, rec.Header().Values("Vary"), "Origin",
			"a response that depends on the Origin must not be cached across origins")
	}

	rec := preflight(t, options, "https://evil.example")
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCORSKeepsTheSingleOriginField(t *testing.T) {
	options := NewCORSOptions()
	options.AllowedOrigin = "https://app.example"

	assert.Equal(t, http.StatusNoContent, preflight(t, options, "https://app.example").Code)
	assert.Equal(t, http.StatusForbidden, preflight(t, options, "https://evil.example").Code)
}

// Browsers reject `Access-Control-Allow-Origin: *` together with credentials,
// and echoing back an arbitrary origin while allowing them would let any site
// act as the logged-in user.
func TestCORSRefusesCredentialsWithAnUnrestrictedOrigin(t *testing.T) {
	options := NewCORSOptions()
	options.AllowCredentials = true

	rec := preflight(t, options, "https://anywhere.example")

	assert.Empty(t, rec.Header().Get(fhttp.HeaderAccessControlAllowCredentials))
}

func TestCORSAllowsCredentialsForAKnownOrigin(t *testing.T) {
	options := NewCORSOptions()
	options.AllowedOrigins = []string{"https://app.example"}
	options.AllowCredentials = true

	rec := preflight(t, options, "https://app.example")

	assert.Equal(t, "true", rec.Header().Get(fhttp.HeaderAccessControlAllowCredentials))
}

func TestCORSRejectsAPreflightWithoutAnOrigin(t *testing.T) {
	assert.Equal(t, http.StatusBadRequest, preflight(t, NewCORSOptions(), "").Code)
}

// Server-to-server traffic carries no Origin and needs no CORS headers.
func TestCORSPassesThroughRequestsWithoutAnOrigin(t *testing.T) {
	rec := httptest.NewRecorder()
	corsHandler(NewCORSOptions()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/chats", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get(fhttp.HeaderAccessControlAllowOrigin))
}

// setDefaultCORSOptions mutated the caller's struct and appended the defaults
// to its slices, so building a handler twice grew those lists each time.
func TestCORSDoesNotMutateTheOptions(t *testing.T) {
	options := NewCORSOptions()
	options.AllowedMethods = []string{http.MethodGet}
	options.AllowedHeaders = []string{"X-Custom"}

	first := preflight(t, options, "https://example.com").Header().Get(fhttp.HeaderAccessControlAllowMethods)
	second := preflight(t, options, "https://example.com").Header().Get(fhttp.HeaderAccessControlAllowMethods)

	assert.Equal(t, first, second)
	assert.Equal(t, []string{http.MethodGet}, options.AllowedMethods, "the caller's slice must be untouched")
	assert.Equal(t, []string{"X-Custom"}, options.AllowedHeaders)
}

func TestCORSCustomHeadersAndMaxAge(t *testing.T) {
	maxAge := 90 * time.Second

	options := NewCORSOptions()
	options.AllowedHeaders = []string{"X-Custom"}
	options.ExposedHeaders = []string{"X-Result"}
	options.MaxAge = &maxAge

	rec := preflight(t, options, "https://example.com")

	assert.Contains(t, rec.Header().Get(fhttp.HeaderAccessControlAllowHeaders), "X-Custom")
	assert.Contains(t, rec.Header().Get(fhttp.HeaderAccessControlAllowHeaders), fhttp.HeaderAuthorization)
	assert.Contains(t, rec.Header().Get(fhttp.HeaderAccessControlExposeHeaders), "X-Result")
	assert.Equal(t, "90", rec.Header().Get(fhttp.HeaderAccessControlMaxAge))
}

func TestCORSMatchesOriginsCaseInsensitively(t *testing.T) {
	options := NewCORSOptions()
	options.AllowedOrigins = []string{"https://App.Example"}

	rec := preflight(t, options, "https://app.example")

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestCORSNilOptions(t *testing.T) {
	require.NotPanics(t, func() {
		rec := preflight(t, nil, "https://example.com")
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

func TestGetUniqueStrings(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, getUniqueStrings([]string{"a", "b", "a"}))
	assert.Empty(t, getUniqueStrings(nil))
	assert.Equal(t, 1, strings.Count(strings.Join(getUniqueStrings([]string{"a", "a"}), ","), "a"))
}

// The comparison is case-insensitive but the browser's check of
// Access-Control-Allow-Origin is not, so echoing the configured spelling made
// the browser reject a request the gateway had allowed — with no 403 in the
// logs to explain it.
func TestCORSEchoesTheRequestOriginSpelling(t *testing.T) {
	options := NewCORSOptions()
	options.AllowedOrigins = []string{"https://App.Example.com"}

	rec := preflight(t, options, "https://app.example.com")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "https://app.example.com", rec.Header().Get(fhttp.HeaderAccessControlAllowOrigin))
}
