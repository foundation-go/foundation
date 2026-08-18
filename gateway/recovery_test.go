package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fctx "github.com/foundation-go/foundation/context"
)

// net/http recovers handler panics on its own, but silently: it drops the
// connection with no response, no log line and no report. A panic in a gateway
// handler was invisible to everyone.
func TestWithRecoveryTurnsAPanicIntoA500(t *testing.T) {
	handler := WithRecovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()

	require.NotPanics(t, func() {
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats", nil))
	})

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, internalErrorBody, rec.Body.String())
}

// The panic value may hold anything, including internal detail. It belongs in
// the log, not in the response.
func TestWithRecoveryDoesNotLeakThePanicValue(t *testing.T) {
	handler := WithRecovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("connection string postgres://user:hunter2@db/app")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats", nil))

	assert.NotContains(t, rec.Body.String(), "hunter2")
}

func TestWithRecoveryLogsTheStack(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	hook := &captureHook{}
	logger.AddHook(hook)

	handler := WithRecovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/chats", nil)
	req = req.WithContext(fctx.WithLogger(req.Context(), logrus.NewEntry(logger)))

	handler.ServeHTTP(httptest.NewRecorder(), req)

	entries := hook.snapshot()
	require.NotEmpty(t, entries)

	stack, ok := entries[0]["stack"].(string)
	require.True(t, ok, "the panic report must carry a stack")
	assert.Contains(t, stack, "recovery_test.go")
}

func TestWithRecoveryPassesThroughNormalRequests(t *testing.T) {
	handler := WithRecovery(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats", nil))

	assert.Equal(t, http.StatusTeapot, rec.Code)
}

// ErrAbortHandler is the documented way for a handler to abandon a response. It
// is not a bug and must keep its net/http semantics.
func TestWithRecoveryRepanicsOnErrAbortHandler(t *testing.T) {
	handler := WithRecovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	})
}
