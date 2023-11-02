package gateway

import (
	"net/http"
	"strconv"
	"strings"

	fhttp "github.com/foundation-go/foundation/http"
	fhydra "github.com/foundation-go/foundation/hydra"
)

// AuthenticationHandler is a function that authenticates the request
type AuthenticationHandler func(token string) (*AuthenticationResult, error)

// AuthenticationResult is the result of an authentication
type AuthenticationResult struct {
	IsAuthenticated bool
	ClientID        string
	UserID          string
	Scope           string
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
	r.Header.Set(fhttp.HeaderXAuthenticated, strconv.FormatBool(result.IsAuthenticated))
	r.Header.Set(fhttp.HeaderXClientID, result.ClientID)
	r.Header.Set(fhttp.HeaderXScope, result.Scope)
	r.Header.Set(fhttp.HeaderXUserID, result.UserID)

	return r
}
