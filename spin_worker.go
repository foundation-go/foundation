package foundation

import (
	"context"
	"time"

	ferr "github.com/foundation-go/foundation/errors"
)

const (
	SpinWorkerDefaultInterval = 5 * time.Millisecond
)

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
	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			default:
				started := time.Now()

				if err := sw.Options.ProcessFunc(ctx); err != nil {
					sw.HandleError(err, "failed to process iteration")
				}

				// Sleep for the remaining time of the interval
				if sw.Options.Interval > 0 {
					time.Sleep(sw.Options.Interval - time.Since(started))
				}
			}
		}
	}()

	<-ctx.Done()

	return nil
}
