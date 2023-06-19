package foundation

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
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
		app.Logger.Fatalf("Failed to start components: %v", err)
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
		if err := server.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				app.Logger.Println("Server stopped")
			} else {
				app.Logger.Fatalf("Failed to start server: %v", err)
			}
		}
	}()

	<-ctx.Done()
	app.Logger.Println("Shutting down server...")

	// Gracefully stop the server
	if err := server.Shutdown(context.Background()); err != nil {
		app.Logger.Fatalf("Failed to gracefully shutdown server: %v", err)
	}

	app.StopComponents()

	app.Logger.Println("Server gracefully stopped")
}
