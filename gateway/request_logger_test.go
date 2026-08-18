package gateway

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fctx "github.com/foundation-go/foundation/context"
	fhttp "github.com/foundation-go/foundation/http"
)

// captureHook collects every emitted entry so that tests can assert on the
// fields that were actually logged.
type captureHook struct {
	mu      sync.Mutex
	entries []logrus.Fields
}

func (h *captureHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h *captureHook) Fire(entry *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	fields := make(logrus.Fields, len(entry.Data))
	for k, v := range entry.Data {
		fields[k] = v
	}
	fields["__msg"] = entry.Message

	h.entries = append(h.entries, fields)

	return nil
}

func (h *captureHook) snapshot() []logrus.Fields {
	h.mu.Lock()
	defer h.mu.Unlock()

	out := make([]logrus.Fields, len(h.entries))
	copy(out, h.entries)

	return out
}

func newTestLogger() (*logrus.Entry, *captureHook) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logger.SetLevel(logrus.DebugLevel)

	hook := &captureHook{}
	logger.AddHook(hook)

	return logrus.NewEntry(logger), hook
}

// WithRequestLogger used to assign the enriched entry back to the *captured*
// `l` variable, which is shared by every in-flight request. Under `go test
// -race` this produced dozens of race reports, and in production it silently
// mixed one request's fields into another request's log lines.
func TestWithRequestLoggerIsConcurrencySafe(t *testing.T) {
	logger, hook := newTestLogger()

	const requests = 64

	handler := WithRequestLogger(logger)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/path-%d", i), nil)
			req.Header.Set(fhttp.HeaderXRequestID, fmt.Sprintf("req-%d", i))

			handler.ServeHTTP(httptest.NewRecorder(), req)
		}(i)
	}
	wg.Wait()

	entries := hook.snapshot()
	require.Len(t, entries, requests*2, "expected a started and a finished line per request")

	// Every line must describe exactly one request: `request_id` and `path`
	// have to agree. They disagree as soon as fields bleed between requests.
	for _, fields := range entries {
		path, _ := fields["path"].(string)
		requestID, _ := fields["request_id"].(string)

		assert.Equal(t, "req-"+path[len("/path-"):], requestID,
			"fields from different requests were mixed: %v", fields)
	}
}

func TestWithRequestLoggerPopulatesContextAndHeaders(t *testing.T) {
	logger, _ := newTestLogger()

	var (
		ctxLogger        *logrus.Entry
		downstreamCorrID string
	)

	handler := WithRequestLogger(logger)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctxLogger = fctx.GetLogger(r.Context())
		downstreamCorrID = r.Header.Get(fhttp.HeaderXCorrelationID)
	}))

	req := httptest.NewRequest(http.MethodGet, "/chats", nil)
	req.Header.Set(fhttp.HeaderXRequestID, "req-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.NotNil(t, ctxLogger)
	assert.Equal(t, "/chats", ctxLogger.Data["path"])
	assert.Equal(t, http.MethodGet, ctxLogger.Data["method"])
	assert.Equal(t, "req-1", ctxLogger.Data["request_id"])

	assert.NotEmpty(t, downstreamCorrID, "correlation ID should reach the handler")
	assert.Equal(t, downstreamCorrID, rec.Header().Get(fhttp.HeaderXCorrelationID))
	assert.Equal(t, "req-1", rec.Header().Get(fhttp.HeaderXRequestID))
}

func TestLoggingResponseWriterRecordsStatus(t *testing.T) {
	logger, hook := newTestLogger()

	handler := WithRequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))

	var statuses []interface{}
	for _, fields := range hook.snapshot() {
		if status, ok := fields["status"]; ok {
			statuses = append(statuses, status)
		}
	}

	assert.Equal(t, []interface{}{http.StatusTeapot}, statuses)
}
