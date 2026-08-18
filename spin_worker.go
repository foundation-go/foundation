package foundation

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	ferr "github.com/foundation-go/foundation/errors"
	fmetrics "github.com/foundation-go/foundation/metrics"
)

const SpinWorkerDefaultInterval = 5 * time.Millisecond

// spinWorkerCancelGrace is how long the worker is given to unwind once the
// drain timeout has already elapsed. It is a variable so that tests can shorten
// it.
var spinWorkerCancelGrace = 5 * time.Second

// SpinWorker is a type of Foundation service.
type SpinWorker struct {
	*Service

	Options *SpinWorkerOptions
}

// InitSpinWorker initializes a new Foundation service in worker mode.
func InitSpinWorker(name string) *SpinWorker {
	return &SpinWorker{
		Init(name),
		NewSpinWorkerOptions(),
	}
}

// SpinWorkerOptions are the options to start a Foundation service in worker mode.
type SpinWorkerOptions struct {
	// ProcessFunc is the function to execute in the loop iteration.
	ProcessFunc func(ctx context.Context) ferr.FoundationError

	// Interval is the interval to run the iteration function. If function execution took less time than the interval,
	// the worker will sleep for the remaining time of the interval. Otherwise, the function will be executed again
	// immediately. Default: 5ms, if constructed with NewSpinWorkerOptions().
	Interval time.Duration

	// ModeName is the name of the worker mode. It will be used in the startup log message. Default: "spin_worker".
	// Meant to be used in custom modes based on the `spin_worker` mode.
	ModeName string

	StartComponentsOptions []StartComponentsOption
}

// NewSpinWorkerOptions returns a new SpinWorkerOptions instance with default values.
func NewSpinWorkerOptions() *SpinWorkerOptions {
	return &SpinWorkerOptions{
		ModeName: "spin_worker",
		Interval: SpinWorkerDefaultInterval,
	}
}

// Start runs the Foundation worker
func (sw *SpinWorker) Start(opts *SpinWorkerOptions) {
	sw.Options = opts

	sw.Service.Start(&StartOptions{
		ModeName:               opts.ModeName,
		StartComponentsOptions: sw.Options.StartComponentsOptions,
		ServiceFunc:            sw.ServiceFunc,
	})
}

// ServiceFunc is the default service function for a worker.
func (sw *SpinWorker) ServiceFunc(ctx context.Context) error {
	done := make(chan struct{})

	go func() {
		defer close(done)

		sw.loop(ctx)
	}()

	<-ctx.Done()

	// Service.Start tears the components down as soon as this returns, so the
	// iteration in flight has to finish first. Without this wait the database
	// pool and the Kafka reader are closed underneath a running ProcessFunc,
	// which fails transactions that were about to commit.
	sw.drain(done)

	return nil
}

// drain waits for the worker loop to finish, within the shutdown budget.
func (sw *SpinWorker) drain(done <-chan struct{}) {
	timeout := sw.shutdownTimeout()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
	case <-timer.C:
		sw.Logger.Warnf(
			"Worker iteration is still running after %s; stopping components anyway", timeout,
		)

		// Give the iteration a last moment to notice, then move on: blocking
		// forever here would just get the process killed by the supervisor.
		grace := time.NewTimer(spinWorkerCancelGrace)
		defer grace.Stop()

		select {
		case <-done:
		case <-grace.C:
			sw.Logger.Error("Worker iteration did not stop; components will be stopped regardless")
		}
	}
}

// runIteration runs a single ProcessFunc call.
//
// A panic here used to kill the process: it happened on the worker goroutine,
// where nothing recovers it and nothing reports it either.
func (sw *SpinWorker) runIteration(ctx context.Context) {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmetrics.Panics.WithLabelValues("worker").Inc()
			sw.CaptureError(
				fmt.Errorf("panic in worker iteration: %v\n%s", recovered, debug.Stack()),
				"",
			)
		}
	}()

	if err := sw.Options.ProcessFunc(ctx); err != nil {
		sw.HandleError(err, "failed to process iteration")
	}
}

// loop runs ProcessFunc until the context is cancelled.
func (sw *SpinWorker) loop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		started := time.Now()

		sw.runIteration(ctx)

		if sw.Options.Interval <= 0 {
			continue
		}

		remaining := sw.Options.Interval - time.Since(started)
		if remaining <= 0 {
			continue
		}

		// Sleep for the remaining time of the interval, but wake up immediately
		// on shutdown instead of sitting through it.
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
