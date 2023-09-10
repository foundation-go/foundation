package gateway

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	fhttp "github.com/ri-nat/foundation/http"
)

// TODO: Make these configurable on the Service level.
var (
	corsAllowedOrigin = "*"
	corsMaxAge        = 24 * time.Hour

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

// WithCORSEnabled is a middleware that enables CORS for the given handler.
func WithCORSEnabled(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(fhttp.HeaderAccessControlAllowOrigin, corsAllowedOrigin)
		w.Header().Set(fhttp.HeaderAccessControlAllowMethods, http.MethodPost)
		w.Header().Set(fhttp.HeaderAccessControlAllowHeaders, strings.Join(corsAllowedHeaders, ", "))
		w.Header().Set(fhttp.HeaderAccessControlExposeHeaders, strings.Join(corsExposedHeaders, ", "))
		w.Header().Set(fhttp.HeaderAccessControlMaxAge, fmt.Sprintf("%d", int(corsMaxAge.Seconds())))

		// If the request is an OPTIONS request, we can safely return here.
		if r.Method == http.MethodOptions {
			return
		}

		handler.ServeHTTP(w, r)
	})
}
