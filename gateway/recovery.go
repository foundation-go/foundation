package gateway

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/getsentry/sentry-go"

	fctx "github.com/foundation-go/foundation/context"
	fmetrics "github.com/foundation-go/foundation/metrics"
)

// internalErrorBody mirrors the shape grpc-gateway uses for error responses, so
// that a panic looks like any other failure to a client.
const internalErrorBody = `{"code":13,"message":"Internal Server Error","details":[]}`

// WithRecovery turns a panic in a downstream handler into a 500 response, a log
// line carrying the stack, and a Sentry report.
//
// net/http already recovers handler panics, but silently: it drops the
// connection without a response, writes nothing to the service log and reports
// nothing. Every panic in a gateway handler was therefore invisible — the
// client saw a reset connection and the operator saw nothing at all.
func WithRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			// ErrAbortHandler is the documented way for a handler to abandon a
			// response. It is not a bug, and net/http suppresses it.
			if recovered == http.ErrAbortHandler { //nolint:errorlint // sentinel compared by identity, as net/http does
				panic(recovered)
			}

			err := fmt.Errorf("panic: %v", recovered)

			fctx.GetLogger(r.Context()).
				WithField("stack", string(debug.Stack())).
				WithError(err).
				Error("Recovered from a panic while handling the request")

			fmetrics.Panics.WithLabelValues("http").Inc()
			sentry.CaptureException(err)

			writeInternalError(w)
		}()

		next.ServeHTTP(w, r)
	})
}

// writeInternalError answers with a generic 500 without leaking the panic value
// to the client.
func writeInternalError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)

	_, _ = w.Write([]byte(internalErrorBody))
}
