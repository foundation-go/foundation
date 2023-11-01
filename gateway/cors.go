package gateway

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	fhttp "github.com/ri-nat/foundation/http"
)

var (
	corsAllowedOrigin = "*"
	corsMaxAge        = 24 * time.Hour
	corsAllowMethods  = []string{http.MethodPost}

	corsAllowedHeaders = []string{
		fhttp.HeaderAccept,
		fhttp.HeaderContentType,
		fhttp.HeaderContentLength,
		fhttp.HeaderAcceptEncoding,
		fhttp.HeaderAuthorization,
		fhttp.HeaderResponseType,
	}

	corsExposedHeaders = []string{
		fhttp.HeaderXCorrelationID,
		fhttp.HeaderXPage,
		fhttp.HeaderXPerPage,
		fhttp.HeaderXTotal,
		fhttp.HeaderXTotalPages,
	}
)

// CORSOptions represents the options for CORS.
type CORSOptions struct {
	// AllowedOrigin is the allowed origin for CORS.
	AllowedOrigin string
	// MaxAge is the max age for CORS.
	MaxAge *time.Duration
	// AllowedHeaders are the allowed headers for CORS.
	AllowedHeaders []string
	// ExposedHeaders are the exposed headers for CORS.
	ExposedHeaders []string
	// AllowedMethods are the allowed methods for CORS.
	AllowedMethods []string
}

func NewCORSOptions() *CORSOptions {
	return &CORSOptions{}
}

func getUniqueStrings(input []string) []string {
	keys := make(map[string]bool)
	list := make([]string, 0, len(input))

	for _, entry := range input {
		if !keys[entry] {
			keys[entry] = true
			list = append(list, entry)
		}
	}

	return list
}

func setDefaultCORSOptions(options *CORSOptions) *CORSOptions {
	if options.AllowedOrigin == "" {
		options.AllowedOrigin = corsAllowedOrigin
	}

	if options.MaxAge == nil {
		options.MaxAge = &corsMaxAge
	}

	options.AllowedHeaders = getUniqueStrings(append(options.AllowedHeaders, corsAllowedHeaders...))
	options.ExposedHeaders = getUniqueStrings(append(options.ExposedHeaders, corsExposedHeaders...))
	options.AllowedMethods = getUniqueStrings(append(options.AllowedMethods, corsAllowMethods...))

	return options
}

// WithCORSEnabled is a middleware that enables CORS for the given handler.
func WithCORSEnabled(options *CORSOptions) func(http.Handler) http.Handler {
	options = setDefaultCORSOptions(options)

	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin == "" {
				if r.Method == http.MethodOptions {
					http.Error(w, "Missing Origin header in preflight request", http.StatusBadRequest)
					return
				}

				// Backend-to-backend request, no CORS needed.
				handler.ServeHTTP(w, r)
				return
			}

			if options.AllowedOrigin != "*" && origin != options.AllowedOrigin {
				http.Error(w, fmt.Sprintf("CORS origin %s not allowed", origin), http.StatusForbidden)
				return
			}

			w.Header().Set(fhttp.HeaderAccessControlAllowOrigin, options.AllowedOrigin)
			w.Header().Set(fhttp.HeaderAccessControlAllowMethods, strings.Join(options.AllowedMethods, ", "))
			w.Header().Set(fhttp.HeaderAccessControlAllowHeaders, strings.Join(options.AllowedHeaders, ", "))
			w.Header().Set(fhttp.HeaderAccessControlExposeHeaders, strings.Join(options.ExposedHeaders, ", "))
			w.Header().Set(fhttp.HeaderAccessControlMaxAge, fmt.Sprintf("%d", int(options.MaxAge.Seconds())))

			// If the request is an OPTIONS request, we can safely return here.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			handler.ServeHTTP(w, r)
		})
	}
}
