package foundation

import (
	"context"
	"time"

	ferr "github.com/foundation-go/foundation/errors"
)

const (
	WorkerDefaultInterval = 5 * time.Millisecond
)

// Worker is a type of Foundation service.
type Worker struct {
	*Service

	Options *WorkerOptions
}

// InitWorker initializes a new Foundation service in worker mode.
func InitWorker(name string) *Worker {
	return &Worker{
		Init(name),
		NewWorkerOptions(),
	}
}

// WorkerOptions are the options to start a Foundation service in worker mode.
type WorkerOptions struct {
	// ProcessFunc is the function to execute in the loop iteration.
	ProcessFunc func(ctx context.Context) ferr.FoundationError

	// Interval is the interval to run the iteration function. If function execution took less time than the interval,
	// the worker will sleep for the remaining time of the interval. Otherwise, the function will be executed again
	// immediately. Default: 5ms, if constructed with NewWorkerOptions().
	Interval time.Duration

	// ModeName is the name of the worker mode. It will be used in the startup log message. Default: "worker".
	// Meant to be used in custom modes based on the `worker` mode.
	ModeName string

	StartComponentsOptions []StartComponentsOption
}

// NewWorkerOptions returns a new WorkerOptions instance with default values.
func NewWorkerOptions() *WorkerOptions {
	return &WorkerOptions{
		ModeName: "worker",
		Interval: WorkerDefaultInterval,
	}
}

// Start runs the Foundation worker
func (w *Worker) Start(opts *WorkerOptions) {
	w.Options = opts

	w.Service.Start(&StartOptions{
		ModeName:               opts.ModeName,
		StartComponentsOptions: w.Options.StartComponentsOptions,
		ServiceFunc:            w.ServiceFunc,
	})
}

// ServiceFunc is the default service function for a worker.
func (w *Worker) ServiceFunc(ctx context.Context) error {
	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			default:
				started := time.Now()

				if err := w.Options.ProcessFunc(ctx); err != nil {
					w.HandleError(err, "failed to process iteration")
				}

				// Sleep for the remaining time of the interval
				if w.Options.Interval > 0 {
					time.Sleep(w.Options.Interval - time.Since(started))
				}
			}
		}
	}()

	<-ctx.Done()

	return nil
}
