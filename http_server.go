package foundation

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/getsentry/sentry-go"
)

// StartHTTPServerOptions are the options to start a Foundation application in server mode.
type StartHTTPServerOptions struct {
	// Handler is the HTTP handler to use.
	Handler http.Handler
}

func NewStartHTTPServerOptions() StartHTTPServerOptions {
	return StartHTTPServerOptions{}
}

// StartHTTPServer starts a Foundation application in HTTP Server mode.
func (app *Application) StartHTTPServer(opts StartHTTPServerOptions) {
	app.logStartup("http")

	// Start common components
	if err := app.StartComponents(); err != nil {
		err = fmt.Errorf("failed to start components: %w", err)
		sentry.CaptureException(err)
		app.Logger.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	port := GetEnvOrInt("PORT", 51051)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: opts.Handler,
	}

	app.Logger.Infof("Listening on http://0.0.0.0:%d", port)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			err = fmt.Errorf("failed to start server: %w", err)
			sentry.CaptureException(err)
			app.Logger.Fatal(err)
		}
	}()

	<-ctx.Done()
	app.Logger.Println("Shutting down server...")

	// Gracefully stop the server
	if err := server.Shutdown(context.Background()); err != nil {
		err = fmt.Errorf("failed to gracefully shutdown server: %w", err)
		sentry.CaptureException(err)
		app.Logger.Fatal(err)
	}

	app.StopComponents()

	app.Logger.Println("Application gracefully stopped")
}
