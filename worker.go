package foundation

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
)

const (
	WorkerDefaultInterval = 5 * time.Millisecond
)

// Worker is a type of Foundation service.
type Worker struct {
	*Service
}

// InitWorker initializes a new Foundation service in worker mode.
func InitWorker(name string) *Worker {
	if name == "" {
		name = "worker"
	}

	return &Worker{
		Init(name),
	}
}

// WorkerOptions are the options to start a Foundation service in worker mode.
type WorkerOptions struct {
	// ProcessFunc is the function to execute in the loop iteration.
	ProcessFunc func(ctx context.Context) FoundationError

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

// Start starts a Foundation worker
func (w *Worker) Start(opts *WorkerOptions) {
	w.logStartup(opts.ModeName)

	// Start common components
	if err := w.StartComponents(opts.StartComponentsOptions...); err != nil {
		err = fmt.Errorf("failed to start components: %w", err)
		sentry.CaptureException(err)
		w.Logger.Fatal(err)
	}

	// Watch for termination signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Run the iteration function in a loop until the service is stopped
	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			default:
				started := time.Now()

				if err := opts.ProcessFunc(ctx); err != nil {
					w.HandleError(err, "failed to process iteration")
				}

				// Sleep for the remaining time of the interval
				if opts.Interval > 0 {
					time.Sleep(opts.Interval - time.Since(started))
				}
			}
		}
	}()

	<-ctx.Done()

	w.Logger.Info("Shutting down service...")

	w.StopComponents()

	w.Logger.Info("Service gracefully stopped")
}
