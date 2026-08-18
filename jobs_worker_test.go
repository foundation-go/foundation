package foundation

import (
	"context"
	"testing"

	"github.com/gocraft/work"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fctx "github.com/foundation-go/foundation/context"
)

func TestJobHandlerRequiresAHandler(t *testing.T) {
	worker := &JobsWorker{Service: newTestService()}

	_, err := worker.jobHandler(context.Background(), "send_email", JobOptions{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no handler")
}

func TestJobHandlerRejectsBothHandlerForms(t *testing.T) {
	worker := &JobsWorker{Service: newTestService()}

	_, err := worker.jobHandler(context.Background(), "send_email", JobOptions{
		Handler:            func(*work.Job) error { return nil },
		HandlerWithContext: func(context.Context, *work.Job) error { return nil },
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pick one")
}

// gocraft/work hands handlers nothing but the job, which left jobs outside
// everything the rest of the framework does with a context: no cancellation, no
// tracing, no correlation ID.
func TestJobHandlerWithContextCarriesCancellationAndCorrelation(t *testing.T) {
	worker := &JobsWorker{Service: newTestService()}

	ctx, cancel := context.WithCancel(context.Background())

	var (
		gotCorrelationID string
		gotCancelled     bool
		gotLogger        bool
	)

	handler, err := worker.jobHandler(ctx, "send_email", JobOptions{
		HandlerWithContext: func(jobCtx context.Context, _ *work.Job) error {
			gotCorrelationID = fctx.GetCorrelationID(jobCtx)
			gotLogger = fctx.GetLogger(jobCtx) != nil

			cancel()
			gotCancelled = jobCtx.Err() != nil

			return nil
		},
	})
	require.NoError(t, err)

	job := &work.Job{
		Name: "send_email",
		Args: map[string]interface{}{CorrelationIDArg: "corr-1"},
	}

	require.NoError(t, handler(job))

	assert.Equal(t, "corr-1", gotCorrelationID)
	assert.True(t, gotCancelled, "the job context must follow the service shutdown")
	assert.True(t, gotLogger)
}

func TestJobHandlerWithoutContextIsPassedThrough(t *testing.T) {
	worker := &JobsWorker{Service: newTestService()}

	called := false
	handler, err := worker.jobHandler(context.Background(), "send_email", JobOptions{
		Handler: func(*work.Job) error {
			called = true
			return nil
		},
	})
	require.NoError(t, err)
	require.NoError(t, handler(&work.Job{Name: "send_email"}))

	assert.True(t, called)
}

func TestNewJobsWorkerOptionsDefaults(t *testing.T) {
	opts := NewJobsWorkerOptions()

	assert.Equal(t, defaultConcurrency, opts.Concurrency)
	assert.NotEmpty(t, opts.Namespace)
}
