package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	fhttp "github.com/foundation-go/foundation/http"
)

// forwardedMetadata runs a request through the strip middleware and the
// grpc-gateway annotation step, returning the metadata a downstream service
// would see.
func forwardedMetadata(t *testing.T, req *http.Request) metadata.MD {
	t.Helper()

	mux := gwruntime.NewServeMux(gwruntime.WithIncomingHeaderMatcher(IncomingHeaderMatcher))

	var md metadata.MD

	handler := StripClientAuthenticationHeaders(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctx, err := gwruntime.AnnotateIncomingContext(r.Context(), mux, r, "/service/Method")
		require.NoError(t, err)

		md, _ = metadata.FromIncomingContext(ctx)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), req)

	return md
}

// grpc-gateway's default header matcher accepts any `Grpc-Metadata-Foo` header
// and forwards `Foo` as gRPC metadata. Deleting `X-User-Id` therefore did not
// stop `Grpc-Metadata-X-User-Id` from arriving downstream as `x-user-id` — the
// key MetadataUnaryInterceptor trusts. The identity fix was bypassable with one
// extra curl flag.
func TestIdentityHeadersCannotBeSmuggledThroughGrpcMetadataAliases(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/chats", nil)
	req.Header.Set("Grpc-Metadata-X-Authenticated", "true")
	req.Header.Set("Grpc-Metadata-X-User-Id", "22222222-2222-2222-2222-222222222222")
	req.Header.Set("Grpc-Metadata-X-Client-Id", "33333333-3333-3333-3333-333333333333")
	req.Header.Set("Grpc-Metadata-X-Scope", "admin")
	req.Header.Set("Grpc-Metadata-X-Metadata", `{"role":"admin"}`)

	md := forwardedMetadata(t, req)

	for _, key := range []string{"x-authenticated", "x-user-id", "x-client-id", "x-scope", "x-metadata"} {
		assert.Empty(t, md.Get(key), "%s must not be forwarded from a client-supplied alias", key)
	}
}

func TestPlainIdentityHeadersAreNotForwardedEither(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/chats", nil)
	for _, header := range fhttp.AuthenticationHeaders {
		req.Header.Set(header, "spoofed")
	}

	md := forwardedMetadata(t, req)

	for _, header := range fhttp.AuthenticationHeaders {
		assert.Empty(t, md.Get(canonicalMetadataKey(header)), "%s must not be forwarded", header)
	}
}

// Non-identity headers still travel, including through the `Grpc-Metadata-`
// alias: only identity is special.
func TestNonIdentityHeadersStillReachTheService(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/chats", nil)
	req.Header.Set(fhttp.HeaderXRequestID, "req-1")
	req.Header.Set("Grpc-Metadata-X-Trace-Flag", "on")

	md := forwardedMetadata(t, req)

	assert.Equal(t, []string{"req-1"}, md.Get("x-request-id"))
	assert.Equal(t, []string{"on"}, md.Get("x-trace-flag"))
}

func TestIncomingHeaderMatcher(t *testing.T) {
	tests := []struct {
		key      string
		wantKey  string
		wantPass bool
	}{
		{key: fhttp.HeaderXUserID, wantKey: fhttp.HeaderXUserID, wantPass: true},
		{key: "x-user-id", wantKey: "x-user-id", wantPass: true},
		{key: fhttp.HeaderXRequestID, wantKey: fhttp.HeaderXRequestID, wantPass: true},

		{key: "Grpc-Metadata-X-User-Id", wantPass: false},
		{key: "grpc-metadata-x-user-id", wantPass: false},
		{key: "GRPC-METADATA-X-AUTHENTICATED", wantPass: false},
		{key: "Grpc-Metadata-X-Scope", wantPass: false},

		// Not an identity header, so the default matcher still handles it.
		{key: "Grpc-Metadata-X-Request-Id", wantKey: "X-Request-Id", wantPass: true},
		{key: "Grpc-Metadata-X-Trace-Flag", wantKey: "X-Trace-Flag", wantPass: true},
		// A near miss must not be mistaken for an alias.
		{key: "X-Grpc-Metadata-X-User-Id", wantPass: false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			key, ok := IncomingHeaderMatcher(tt.key)

			assert.Equal(t, tt.wantPass, ok)
			if tt.wantPass {
				assert.Equal(t, tt.wantKey, key)
			}
		})
	}
}

func TestOutgoingHeaderMatcherMirrorsIncoming(t *testing.T) {
	_, ok := OutgoingHeaderMatcher("Grpc-Metadata-X-User-Id")
	assert.False(t, ok)

	key, ok := OutgoingHeaderMatcher(fhttp.HeaderXCorrelationID)
	assert.True(t, ok)
	assert.Equal(t, fhttp.HeaderXCorrelationID, key)
}

// canonicalMetadataKey lowercases a header name the way gRPC metadata does.
func canonicalMetadataKey(header string) string {
	out := make([]byte, len(header))
	for i := 0; i < len(header); i++ {
		c := header[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}

	return string(out)
}
