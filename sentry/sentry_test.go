package sentry

import (
	"errors"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSDK swaps the Sentry SDK seams for the duration of a test and records the
// calls made through them.
func stubSDK(t *testing.T) (captured *[]error, flushed *[]time.Duration) {
	t.Helper()

	var (
		gotErrors   []error
		gotTimeouts []time.Duration
	)

	origCapture, origFlush := captureException, flush
	t.Cleanup(func() { captureException, flush = origCapture, origFlush })

	captureException = func(err error) *sentry.EventID {
		gotErrors = append(gotErrors, err)
		return nil
	}
	flush = func(timeout time.Duration) bool {
		gotTimeouts = append(gotTimeouts, timeout)
		return true
	}

	return &gotErrors, &gotTimeouts
}

// `sentry.Flush` takes a time.Duration, and the component used to call
// `sentry.Flush(2)` — two nanoseconds. Buffered events were therefore dropped
// on every shutdown.
func TestStopFlushesWithAUsefulTimeout(t *testing.T) {
	_, flushed := stubSDK(t)

	require.NoError(t, NewComponent("dsn").Stop())

	require.Len(t, *flushed, 1)
	assert.GreaterOrEqual(t, (*flushed)[0], time.Second,
		"a sub-second flush timeout cannot deliver anything")
	assert.Equal(t, DefaultFlushTimeout, (*flushed)[0])
}

func TestStopUsesTheConfiguredTimeout(t *testing.T) {
	_, flushed := stubSDK(t)

	require.NoError(t, NewComponent("dsn", WithFlushTimeout(7*time.Second)).Stop())

	require.Len(t, *flushed, 1)
	assert.Equal(t, 7*time.Second, (*flushed)[0])
}

func TestFlushTimeoutFallsBackForNonPositiveValues(t *testing.T) {
	assert.Equal(t, DefaultFlushTimeout, NewComponent("dsn", WithFlushTimeout(0)).FlushTimeout())
	assert.Equal(t, DefaultFlushTimeout, NewComponent("dsn", WithFlushTimeout(-1)).FlushTimeout())
}

func TestCaptureAndFlush(t *testing.T) {
	captured, flushed := stubSDK(t)

	err := errors.New("boom")
	CaptureAndFlush(err, 3*time.Second)

	require.Len(t, *captured, 1)
	assert.Equal(t, err, (*captured)[0])

	require.Len(t, *flushed, 1)
	assert.Equal(t, 3*time.Second, (*flushed)[0])
}

func TestCaptureAndFlushIgnoresNilError(t *testing.T) {
	captured, flushed := stubSDK(t)

	CaptureAndFlush(nil, time.Second)

	assert.Empty(t, *captured)
	assert.Empty(t, *flushed)
}

func TestCaptureAndFlushFallsBackForNonPositiveTimeout(t *testing.T) {
	_, flushed := stubSDK(t)

	CaptureAndFlush(errors.New("boom"), 0)

	require.Len(t, *flushed, 1)
	assert.Equal(t, DefaultFlushTimeout, (*flushed)[0])
}

func TestNewComponentDefaults(t *testing.T) {
	c := NewComponent("dsn")

	assert.Equal(t, "dsn", c.DSN)
	assert.Equal(t, ComponentName, c.Name())
	assert.Equal(t, DefaultFlushTimeout, c.FlushTimeout())
	assert.True(t, c.attachStacktrace)
	assert.NoError(t, c.Health())
}

func TestNewComponentOptions(t *testing.T) {
	c := NewComponent("dsn",
		WithEnvironment("production"),
		WithRelease("v1.2.3"),
		WithAttachStacktrace(false),
	)

	assert.Equal(t, "production", c.environment)
	assert.Equal(t, "v1.2.3", c.release)
	assert.False(t, c.attachStacktrace)
}
