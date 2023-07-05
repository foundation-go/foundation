package gateway

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	hydra "github.com/ory/hydra-client-go/v2"
	kratos "github.com/ory/kratos-client-go"

	fhttp "github.com/ri-nat/foundation/http"
)

// AuthenticationHandler is a function that authenticates the request
type AuthenticationHandler func(token string) (*AuthenticationResult, error)

// AuthenticationResult is the result of an authentication
type AuthenticationResult struct {
	IsAuthenticated bool
	UserID          string
}

// WithHydraAuthenticationDetails is a middleware that fetches the authentication details using ORY Hydra
func WithHydraAuthenticationDetails(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WithAuthenticationDetails(handler, func(token string) (*AuthenticationResult, error) {
			hydraAdminURL := os.Getenv("HYDRA_ADMIN_URL")
			if hydraAdminURL == "" {
				return nil, errors.New("HYDRA_ADMIN_URL is not set")
			}

			// Create a new Hydra SDK client
			config := hydra.NewConfiguration()
			config.Servers = hydra.ServerConfigurations{
				{URL: hydraAdminURL},
			}
			client := hydra.NewAPIClient(config)

			// Authenticate the token using ORY Hydra
			req := client.OAuth2Api.IntrospectOAuth2Token(r.Context())
			req = req.Token(token)
			resp, _, err := client.OAuth2Api.IntrospectOAuth2TokenExecute(req)
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
				UserID:          resp.GetSub(),
			}, nil
		}).ServeHTTP(w, r)
	})
}

// WithKratosAuthenticationDetails is a middleware that fetches the authentication details using ORY Kratos
func WithKratosAuthenticationDetails(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WithAuthenticationDetails(handler, func(token string) (*AuthenticationResult, error) {
			kratosFrontendURL := os.Getenv("KRATOS_FRONTEND_URL")
			if kratosFrontendURL == "" {
				return nil, errors.New("KRATOS_FRONTEND_URL is not set")
			}

			// Create a new Kratos SDK client
			cfg := kratos.NewConfiguration()
			cfg.Servers = append(cfg.Servers, kratos.ServerConfiguration{
				URL: kratosFrontendURL,
			})
			client := kratos.NewAPIClient(cfg)

			// Authenticate the token using ORY Kratos
			req := client.FrontendApi.ToSession(r.Context())
			req.XSessionToken(token)
			resp, _, err := client.FrontendApi.ToSessionExecute(req)
			if err != nil {
				return nil, err
			}

			// Check if the token is valid
			if !resp.GetActive() {
				return &AuthenticationResult{}, nil
			}

			// Return the authentication result
			return &AuthenticationResult{
				IsAuthenticated: true,
				UserID:          resp.Identity.Id,
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

		r = setAuthHeaders(r, result.IsAuthenticated, result.UserID)
		// Continue to the next handler
		handler.ServeHTTP(w, r)
	})
}

// WithAuthentication is a middleware that forces the request to be authenticated
func WithAuthentication(handler http.Handler, except []string) http.Handler {
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

func setAuthHeaders(r *http.Request, isAuthenticated bool, userID string) *http.Request {
	r.Header.Set(fhttp.HeaderXAuthenticated, strconv.FormatBool(isAuthenticated))
	r.Header.Set(fhttp.HeaderXUserID, userID)

	return r
}
