package foundation

import (
	"errors"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ferr "github.com/foundation-go/foundation/errors"
)

type levelHook struct {
	entries []*logrus.Entry
}

func (h *levelHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h *levelHook) Fire(entry *logrus.Entry) error {
	h.entries = append(h.entries, entry)

	return nil
}

func serviceWithHook() (*Service, *levelHook) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	hook := &levelHook{}
	logger.AddHook(hook)

	return &Service{
		Name:   "test",
		Config: &Config{Sentry: &SentryConfig{}},
		Logger: logrus.NewEntry(logger),
	}, hook
}

// HandleError only logged *InternalError. Every other Foundation error type
// was discarded, and this is the only error handler the spin worker has.
func TestHandleErrorLogsEveryErrorType(t *testing.T) {
	tests := []struct {
		name string
		err  ferr.FoundationError
	}{
		{"internal", ferr.NewInternalError(errors.New("boom"), "context")},
		{"not found", ferr.NewNotFoundError(nil, "chat", "1")},
		{"permission denied", ferr.NewPermissionDeniedError("read", "chat", "1")},
		{"unauthenticated", ferr.NewUnauthenticatedError("no token")},
		{"stale object", ferr.NewStaleObjectError("chat", "1", 1, 2)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, hook := serviceWithHook()

			svc.HandleError(tt.err, "failed to process iteration")

			require.Len(t, hook.entries, 1, "the error must be logged")
			assert.Equal(t, logrus.ErrorLevel, hook.entries[0].Level)
			assert.Contains(t, hook.entries[0].Message, "failed to process iteration")
		})
	}
}

func TestHandleErrorIgnoresNil(t *testing.T) {
	svc, hook := serviceWithHook()

	svc.HandleError(nil, "prefix")

	assert.Empty(t, hook.entries)
}

// A nil cause used to make formatting the error dereference nil.
func TestFoundationErrorsToleratePartialConstruction(t *testing.T) {
	assert.NotPanics(t, func() {
		assert.Contains(t, ferr.NewNotFoundError(nil, "chat", "1").Error(), "chat/1")
		assert.NotEmpty(t, ferr.NewInternalError(nil, "something went wrong").Error())
		assert.Equal(t, "unknown error", (&ferr.BaseError{}).Error())
	})
}

// A Foundation error used to be a dead end for errors.Is: the wrapped cause was
// unreachable, so `errors.Is(err, pgx.ErrNoRows)` could never be true.
func TestFoundationErrorsUnwrap(t *testing.T) {
	cause := errors.New("no rows in result set")

	notFound := ferr.NewNotFoundError(cause, "chat", "1")
	assert.ErrorIs(t, notFound, cause)

	internal := ferr.NewInternalError(cause, "failed to load the chat")
	assert.ErrorIs(t, internal, cause)
}

func TestWrapError(t *testing.T) {
	cause := errors.New("boom")

	assert.Equal(t, "prefix: boom", wrapError(cause, "prefix").Error())
	assert.Equal(t, "boom", wrapError(cause, "").Error())
	assert.Equal(t, "prefix", wrapError(nil, "prefix").Error())
	assert.Equal(t, "unknown error", wrapError(nil, "").Error())

	assert.ErrorIs(t, wrapError(cause, "prefix"), cause)
}

func TestCaptureErrorLogs(t *testing.T) {
	svc, hook := serviceWithHook()

	svc.CaptureError(errors.New("boom"), "while doing the thing")

	require.Len(t, hook.entries, 1)
	assert.Equal(t, logrus.ErrorLevel, hook.entries[0].Level)
	assert.Contains(t, hook.entries[0].Message, "while doing the thing: boom")
}
