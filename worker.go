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
func NewWorkerOptions() WorkerOptions {
	return WorkerOptions{
		ModeName: "worker",
		Interval: WorkerDefaultInterval,
	}
}

// StartWorker starts a Foundation service in worker mode.
func (s *Service) StartWorker(opts WorkerOptions) {
	s.logStartup(opts.ModeName)

	// Start common components
	if err := s.StartComponents(opts.StartComponentsOptions...); err != nil {
		err = fmt.Errorf("failed to start components: %w", err)
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
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
					s.HandleError(err, "failed to process iteration")
				}

				// Sleep for the remaining time of the interval
				if opts.Interval > 0 {
					time.Sleep(opts.Interval - time.Since(started))
				}
			}
		}
	}()

	<-ctx.Done()

	s.Logger.Info("Shutting down service...")

	s.StopComponents()

	s.Logger.Info("Service gracefully stopped")
}
