package gateway

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	fhttp "github.com/foundation-go/foundation/http"
)

// AnyOrigin allows requests from any origin.
const AnyOrigin = "*"

var (
	corsMaxAge = 24 * time.Hour

	// corsAllowMethods is what a JSON API over grpc-gateway actually needs.
	//
	// The default used to be POST alone, so a browser refused every GET, PUT,
	// PATCH and DELETE the API exposed unless the service listed them itself.
	corsAllowMethods = []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	}

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
		fhttp.HeaderXRequestID,
	}
)

// CORSOptions represents the options for CORS.
type CORSOptions struct {
	// AllowedOrigin is a single allowed origin, or "*" for any. Prefer
	// AllowedOrigins, which takes a list.
	AllowedOrigin string
	// AllowedOrigins is the set of origins permitted to call this gateway. An
	// empty list means any origin.
	AllowedOrigins []string
	// MaxAge is the max age for CORS.
	MaxAge *time.Duration
	// AllowedHeaders are the allowed headers for CORS.
	AllowedHeaders []string
	// ExposedHeaders are the exposed headers for CORS.
	ExposedHeaders []string
	// AllowedMethods are the allowed methods for CORS.
	AllowedMethods []string
	// AllowCredentials permits the browser to send cookies and HTTP
	// authentication with cross-origin requests.
	//
	// It cannot be combined with an unrestricted origin: browsers reject
	// `Access-Control-Allow-Origin: *` together with credentials, and echoing
	// back an arbitrary origin while allowing credentials would let any site
	// act as the logged-in user.
	AllowCredentials bool
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

// resolved is a normalised copy of the options.
//
// Normalising into a copy matters: setDefaultCORSOptions used to mutate the
// caller's struct in place and append the defaults to its slices, so building a
// handler twice from the same options grew those lists each time.
type resolved struct {
	origins          []string
	allowAnyOrigin   bool
	maxAge           time.Duration
	allowedHeaders   string
	exposedHeaders   string
	allowedMethods   string
	allowCredentials bool
}

func resolveCORSOptions(options *CORSOptions) *resolved {
	if options == nil {
		options = NewCORSOptions()
	}

	origins := options.AllowedOrigins
	if options.AllowedOrigin != "" {
		origins = append(append([]string{}, origins...), options.AllowedOrigin)
	}

	allowAny := len(origins) == 0
	for _, origin := range origins {
		if origin == AnyOrigin {
			allowAny = true
		}
	}

	maxAge := corsMaxAge
	if options.MaxAge != nil {
		maxAge = *options.MaxAge
	}

	return &resolved{
		origins:        getUniqueStrings(origins),
		allowAnyOrigin: allowAny,
		maxAge:         maxAge,
		allowedHeaders: strings.Join(getUniqueStrings(append(append([]string{}, options.AllowedHeaders...), corsAllowedHeaders...)), ", "),
		exposedHeaders: strings.Join(getUniqueStrings(append(append([]string{}, options.ExposedHeaders...), corsExposedHeaders...)), ", "),
		allowedMethods: strings.Join(getUniqueStrings(append(append([]string{}, options.AllowedMethods...), corsAllowMethods...)), ", "),
		// Credentials with an unrestricted origin is not a configuration a
		// browser will accept, and honouring it would be a vulnerability.
		allowCredentials: options.AllowCredentials && !allowAny,
	}
}

// allows reports whether an origin may call this gateway, and what to echo back.
func (r *resolved) allows(origin string) (string, bool) {
	if r.allowAnyOrigin {
		return AnyOrigin, true
	}

	for _, allowed := range r.origins {
		if strings.EqualFold(allowed, origin) {
			// Echo what the browser sent, not how it was configured: the
			// comparison is case-insensitive but the browser's check of
			// Access-Control-Allow-Origin is not, so answering with a
			// differently-cased spelling fails the request with no 403 to
			// explain it.
			return origin, true
		}
	}

	return "", false
}

// WithCORSEnabled is a middleware that enables CORS for the given handler.
func WithCORSEnabled(options *CORSOptions) func(http.Handler) http.Handler {
	resolved := resolveCORSOptions(options)

	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin == "" && r.Method == http.MethodOptions {
				http.Error(w, "Missing Origin header in preflight request", http.StatusBadRequest)

				return
			}

			// A server-to-server request carries no Origin and needs no CORS
			// headers.
			if origin == "" {
				handler.ServeHTTP(w, r)

				return
			}

			allowedOrigin, ok := resolved.allows(origin)
			if !ok {
				http.Error(w, fmt.Sprintf("CORS origin %s not allowed", origin), http.StatusForbidden)

				return
			}

			// The response depends on the request's Origin, so it must not be
			// cached and served to a different one.
			if !resolved.allowAnyOrigin {
				w.Header().Add("Vary", "Origin")
			}

			w.Header().Set(fhttp.HeaderAccessControlAllowOrigin, allowedOrigin)
			w.Header().Set(fhttp.HeaderAccessControlAllowMethods, resolved.allowedMethods)
			w.Header().Set(fhttp.HeaderAccessControlAllowHeaders, resolved.allowedHeaders)
			w.Header().Set(fhttp.HeaderAccessControlExposeHeaders, resolved.exposedHeaders)
			w.Header().Set(fhttp.HeaderAccessControlMaxAge, fmt.Sprintf("%d", int(resolved.maxAge.Seconds())))

			if resolved.allowCredentials {
				w.Header().Set(fhttp.HeaderAccessControlAllowCredentials, "true")
			}

			// If the request is an OPTIONS request, we can safely return here.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)

				return
			}

			handler.ServeHTTP(w, r)
		})
	}
}
