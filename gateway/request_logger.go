package gateway

import (
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	fctx "github.com/ri-nat/foundation/context"
	fhttp "github.com/ri-nat/foundation/http"

	"github.com/google/uuid"
)

// loggingResponseWriter is an http.ResponseWriter that tracks the status code of the response.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// NewLoggingResponseWriter creates a new loggingResponseWriter that wraps the provided http.ResponseWriter.
// If WriteHeader is not called, the response will implicitly return a status code of 200 OK.
func NewLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

// WriteHeader sets the status code of the response and calls the underlying ResponseWriter's WriteHeader method.
func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// withRequestLogger is an HTTP handler that wraps the provided handler and logs
// request and response information, as well as updating prometheus metrics. It
// generates a unique request ID for each request and adds it to the request
// header and HTTP response header.
func WithRequestLogger(handler http.Handler) http.Handler {
	// Register the prometheus metrics we want to track
	// prometheus.MustRegister(httpResponseTimeMetric, httpResponseCountMetric)

	// Return an HTTP handler that wraps the original handler
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// Record the start time of the request
		started := time.Now()

		// Generate the request ID
		requestID := uuid.New().String()
		// pass it down to app
		request.Header.Set(fhttp.HeaderXCorrelationID, requestID)
		// write it to HTTP response
		writer.Header().Set(fhttp.HeaderXCorrelationID, requestID)

		// Add the logger to the request context
		l := log.WithField("request_id", requestID).WithField("path", request.URL.Path)
		ctx := fctx.SetLogger(request.Context(), l)
		request = request.WithContext(ctx)

		// Wrap the response writer with our logging response writer
		lrw := NewLoggingResponseWriter(writer)

		// Serve the request with the wrapped response writer
		l.Infoln("Request started")
		handler.ServeHTTP(lrw, request)

		// Calculate the duration of the request
		duration := time.Since(started)
		l.WithField("duration", duration).Infoln("Request finished")
	})
}
