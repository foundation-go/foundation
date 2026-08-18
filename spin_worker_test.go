package foundation

import (
	"context"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ferr "github.com/foundation-go/foundation/errors"
)

func newTestSpinWorker(opts *SpinWorkerOptions) *SpinWorker {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	return &SpinWorker{
		Service: &Service{
			Name:   "test",
			Config: &Config{Sentry: &SentryConfig{}, ShutdownTimeout: time.Second},
			Logger: logrus.NewEntry(logger),
		},
		Options: opts,
	}
}

// ServiceFunc used to return the moment the context was cancelled, while the
// worker goroutine was still inside ProcessFunc. Service.Start then closed the
// database pool and the Kafka reader underneath it.
func TestServiceFuncWaitsForTheIterationInFlight(t *testing.T) {
	var (
		entered  = make(chan struct{})
		release  = make(chan struct{})
		finished atomic.Bool
	)

	worker := newTestSpinWorker(&SpinWorkerOptions{
		Interval: time.Millisecond,
		ProcessFunc: func(context.Context) ferr.FoundationError {
			select {
			case <-entered:
				// Only the first iteration blocks.
			default:
				close(entered)
				<-release
				finished.Store(true)
			}

			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())

	returned := make(chan struct{})
	go func() {
		defer close(returned)
		assert.NoError(t, worker.ServiceFunc(ctx))
	}()

	<-entered
	cancel()

	select {
	case <-returned:
		t.Fatal("ServiceFunc returned while an iteration was still running")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("ServiceFunc did not return after the iteration finished")
	}

	assert.True(t, finished.Load())
}

// A worker whose iteration never finishes must not hold the shutdown forever.
func TestServiceFuncGivesUpOnAStuckIteration(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	orig := spinWorkerCancelGrace
	spinWorkerCancelGrace = 20 * time.Millisecond
	t.Cleanup(func() { spinWorkerCancelGrace = orig })

	worker := newTestSpinWorker(&SpinWorkerOptions{
		Interval: time.Millisecond,
		ProcessFunc: func(context.Context) ferr.FoundationError {
			<-release
			return nil
		},
	})
	worker.Config.ShutdownTimeout = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	returned := make(chan struct{})
	go func() {
		defer close(returned)
		assert.NoError(t, worker.ServiceFunc(ctx))
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-returned:
	case <-time.After(spinWorkerCancelGrace + 2*time.Second):
		t.Fatal("ServiceFunc never gave up on the stuck iteration")
	}
}

// The pause between iterations used to be an uncancellable time.Sleep.
func TestLoopWakesUpImmediatelyOnShutdown(t *testing.T) {
	var iterations atomic.Int64

	worker := newTestSpinWorker(&SpinWorkerOptions{
		Interval: time.Hour,
		ProcessFunc: func(context.Context) ferr.FoundationError {
			iterations.Add(1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())

	returned := make(chan struct{})
	go func() {
		defer close(returned)
		assert.NoError(t, worker.ServiceFunc(ctx))
	}()

	// Let the first iteration run and enter the hour-long pause.
	require.Eventually(t, func() bool { return iterations.Load() >= 1 }, time.Second, time.Millisecond)

	cancel()

	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("the worker sat through its interval instead of shutting down")
	}
}

func TestLoopStopsWithoutRunningWhenAlreadyCancelled(t *testing.T) {
	var iterations atomic.Int64

	worker := newTestSpinWorker(&SpinWorkerOptions{
		ProcessFunc: func(context.Context) ferr.FoundationError {
			iterations.Add(1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, worker.ServiceFunc(ctx))

	assert.Zero(t, iterations.Load())
}

func TestShutdownTimeoutFallsBackToDefault(t *testing.T) {
	assert.Equal(t, DefaultShutdownTimeout, (&Service{}).shutdownTimeout())
	assert.Equal(t, DefaultShutdownTimeout, (&Service{Config: &Config{}}).shutdownTimeout())
	assert.Equal(t, time.Minute, (&Service{Config: &Config{ShutdownTimeout: time.Minute}}).shutdownTimeout())
}
