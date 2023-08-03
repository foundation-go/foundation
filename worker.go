package foundation

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
)

// StartWorkerOptions are the options to start a Foundation application in worker mode.
type StartWorkerOptions struct {
	// ProcessFunc is the function to execute in the loop iteration.
	ProcessFunc func(ctx context.Context) FoundationError

	// Interval is the interval to run the iteration function. If function execution took less time than the interval,
	// the worker will sleep for the remaining time of the interval. Otherwise, the function will be executed again
	// immediately.
	Interval time.Duration

	// ModeName is the name of the worker mode. It will be used in the startup log message. Default: "worker".
	// Meant to be used in custom modes based on the `worker` mode.
	ModeName string

	StartComponentsOptions []StartComponentsOption
}

func NewStartWorkerOptions() StartWorkerOptions {
	return StartWorkerOptions{
		ModeName: "worker",
		Interval: 5 * time.Millisecond,
	}
}

// StartWorker starts a Foundation application in worker mode.
func (app *Application) StartWorker(opts StartWorkerOptions) {
	app.logStartup(opts.ModeName)

	// Start common components
	if err := app.StartComponents(opts.StartComponentsOptions...); err != nil {
		err = fmt.Errorf("failed to start components: %w", err)
		sentry.CaptureException(err)
		app.Logger.Fatal(err)
	}

	// Watch for termination signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Run the iteration function in a loop until the application is stopped
	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			default:
				started := time.Now()

				if err := opts.ProcessFunc(ctx); err != nil {
					app.HandleError(err, "failed to process iteration")
				}

				// Sleep for the remaining time of the interval
				if opts.Interval > 0 {
					time.Sleep(opts.Interval - time.Since(started))
				}
			}
		}
	}()

	<-ctx.Done()

	app.Logger.Infof("Shutting down %s...", opts.ModeName)

	app.StopComponents()

	app.Logger.Println("Application gracefully stopped")
}
