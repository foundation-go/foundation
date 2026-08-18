package grpc

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	fctx "github.com/foundation-go/foundation/context"
)

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

// LoggingUnaryInterceptor used to assign the enriched entry back to the
// *captured* `log` variable, shared by every in-flight call.
func TestLoggingUnaryInterceptorIsConcurrencySafe(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	hook := &captureHook{}
	logger.AddHook(hook)

	interceptor := LoggingUnaryInterceptor(logrus.NewEntry(logger))

	handler := func(context.Context, interface{}) (interface{}, error) { return "ok", nil }

	const calls = 64

	var wg sync.WaitGroup
	for i := 0; i < calls; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			ctx := fctx.WithCorrelationID(context.Background(), fmt.Sprintf("corr-%d", i))
			info := &grpc.UnaryServerInfo{FullMethod: fmt.Sprintf("/svc/Method%d", i)}

			_, err := interceptor(ctx, nil, info, handler)
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	entries := hook.snapshot()
	require.Len(t, entries, calls*2, "expected a started and a finished line per call")

	// `method` and `correlation_id` are set from the same call and must agree.
	for _, fields := range entries {
		method, _ := fields["method"].(string)
		correlationID, _ := fields["correlation_id"].(string)

		assert.Equal(t, "corr-"+method[len("/svc/Method"):], correlationID,
			"fields from different calls were mixed: %v", fields)
	}
}

func TestLoggingUnaryInterceptorPutsLoggerInContext(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	interceptor := LoggingUnaryInterceptor(logrus.NewEntry(logger))

	var got *logrus.Entry
	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		got = fctx.GetLogger(ctx)
		return "ok", nil
	}

	ctx := fctx.WithCorrelationID(context.Background(), "corr-1")
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}, handler)
	require.NoError(t, err)

	require.NotNil(t, got)
	assert.Equal(t, "/svc/Method", got.Data["method"])
	assert.Equal(t, "corr-1", got.Data["correlation_id"])
}

// The interceptor reads the correlation ID from the context; a context without
// one must not bring the call down.
func TestLoggingUnaryInterceptorHandlesMissingCorrelationID(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	interceptor := LoggingUnaryInterceptor(logrus.NewEntry(logger))
	handler := func(context.Context, interface{}) (interface{}, error) { return "ok", nil }

	assert.NotPanics(t, func() {
		_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/M"}, handler)
		assert.NoError(t, err)
	})
}
