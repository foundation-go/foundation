package foundation

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

// StartWorkerOptions are the options to start a Foundation application in worker mode.
type StartWorkerOptions struct {
	// ProcessFunc is the function to execute in the loop iteration.
	ProcessFunc func(ctx context.Context) error

	// Interval is the interval to run the iteration function. If function execution took less time than the interval,
	// the worker will sleep for the remaining time of the interval. Otherwise, the function will be executed again
	// immediately.
	Interval time.Duration
}

func NewStartWorkerOptions() StartWorkerOptions {
	return StartWorkerOptions{}
}

// StartWorker starts a Foundation application in worker mode.
func (app *Application) StartWorker(opts StartWorkerOptions) {
	logApplicationStartup("worker")

	// Start common components
	if err := app.StartComponents(); err != nil {
		log.Fatalf("Failed to start components: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Run the iteration function in a loop until the application is stopped
Loop:
	for {
		select {
		case <-ctx.Done():
			break Loop
		default:
			started := time.Now()

			if err := opts.ProcessFunc(ctx); err != nil {
				log.Errorf("Failed to run iteration: %v", err)
				continue
			}

			// Sleep for the remaining time of the interval
			time.Sleep(opts.Interval - time.Since(started))
		}
	}

	log.Println("Shutting down worker...")

	app.StopComponents()

	log.Println("Worker gracefully stopped")
}
