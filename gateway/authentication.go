package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	fctx "github.com/foundation-go/foundation/context"
	fhttp "github.com/foundation-go/foundation/http"
	fhydra "github.com/foundation-go/foundation/hydra"
)

// AuthenticationHandler is a function that authenticates the request
type AuthenticationHandler func(token string) (*AuthenticationResult, error)

// AuthenticationResult is the result of an authentication
type AuthenticationResult struct {
	// IsAuthenticated is true if the request is authenticated
	IsAuthenticated bool
	// ClientID is the OAuth2 client ID of the authenticated user (if any)
	ClientID string
	// UserID is the user ID of the authenticated user
	UserID string
	// Scope is the OAuth2 scope of the authenticated user
	Scope string
	// Metadata is the additional metadata for the authenticated user
	Metadata map[string]string
}

// WithHydraAuthenticationDetails is a middleware that fetches the authentication details using ORY Hydra
func WithHydraAuthenticationDetails(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WithAuthenticationDetails(handler, func(token string) (*AuthenticationResult, error) {
			resp, err := fhydra.IntrospectedOAuth2Token(r.Context(), token)
			if err != nil {
				return nil, err
			}

			// Check if the token is valid
			if !resp.Active {
				return &AuthenticationResult{}, nil
			}

			// Return the authentication result
			return &AuthenticationResult{
				IsAuthenticated: true,
				ClientID:        resp.GetClientId(),
				UserID:          resp.GetSub(),
				Scope:           resp.GetScope(),
			}, nil
		}).ServeHTTP(w, r)
	})
}

// StripClientAuthenticationHeaders removes every identity-bearing Foundation
// header from the incoming request.
//
// The gateway derives `X-Authenticated`, `X-User-Id`, `X-Client-Id`, `X-Scope`
// and `X-Metadata` from the authentication result and forwards them to
// downstream services, which trust them unconditionally. Without this
// middleware a client can simply send `X-Authenticated: true` together with a
// victim's `X-User-Id` and be treated as that user — both by the gateway's own
// `WithAuthentication` and by every service behind it.
//
// Foundation installs this as the first middleware in the chain. Disable it
// only when the gateway sits behind a trusted proxy that sets these headers
// itself, via `GatewayOptions.TrustInboundAuthenticationHeaders`.
//
// Both spellings are removed: the plain header and its `Grpc-Metadata-` alias,
// which grpc-gateway would otherwise forward as the same gRPC metadata key.
func StripClientAuthenticationHeaders(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, header := range fhttp.AuthenticationHeaders {
			r.Header.Del(header)
			r.Header.Del("Grpc-Metadata-" + header)
		}

		handler.ServeHTTP(w, r)
	})
}

// WithAuthenticationDetails is a middleware that fetches the authentication details using the given authentication function
func WithAuthenticationDetails(handler http.Handler, authenticate AuthenticationHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the token from the request header
		token := r.Header.Get(fhttp.HeaderAuthorization)
		// Strip any Bearer prefix
		tokenParts := strings.Split(token, " ")
		token = tokenParts[len(tokenParts)-1]

		// Authenticate the token
		result, err := authenticate(token)
		if err != nil {
			// An authentication backend that is down must not fail silently:
			// every request would simply become unauthenticated with nothing
			// in the logs to explain the sudden wave of 401s.
			fctx.GetLogger(r.Context()).WithError(err).Error("Failed to authenticate request")
		}

		// If the authentication failed and no result was returned, set the result to an empty AuthenticationResult
		if result == nil {
			result = &AuthenticationResult{}
		}

		r = setAuthHeaders(r, result)
		// Continue to the next handler
		handler.ServeHTTP(w, r)
	})
}

// WithAuthentication is a middleware that forces the request to be authenticated
func WithAuthentication(except []string) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the request path is in the exceptions list
			for _, path := range except {
				if path == r.URL.Path {
					handler.ServeHTTP(w, r)
					return
				}
			}

			if r.Header.Get(fhttp.HeaderXAuthenticated) != "true" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			handler.ServeHTTP(w, r)
		})
	}
}

func setAuthHeaders(r *http.Request, result *AuthenticationResult) *http.Request {
	metadata, err := json.Marshal(result.Metadata)
	if err != nil {
		metadata = []byte("{}")
	}

	r.Header.Set(fhttp.HeaderXAuthenticated, strconv.FormatBool(result.IsAuthenticated))
	r.Header.Set(fhttp.HeaderXClientID, result.ClientID)
	r.Header.Set(fhttp.HeaderXScope, result.Scope)
	r.Header.Set(fhttp.HeaderXMetadata, string(metadata))
	r.Header.Set(fhttp.HeaderXUserID, result.UserID)

	return r
}
