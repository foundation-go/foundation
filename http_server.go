package foundation

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
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
func (a Application) StartHTTPServer(opts StartHTTPServerOptions) {
	logApplicationStartup("http")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	port := GetEnvOrInt("PORT", 51051)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: opts.Handler,
	}

	log.Infof("Listening on http://0.0.0.0:%d", port)

	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				log.Println("Server stopped")
			} else {
				log.Fatalf("Failed to start server: %v", err)
			}
		}
	}()

	// Metrics
	go StartMetricsServer()

	<-ctx.Done()
	log.Println("Shutting down server...")

	// Gracefully stop the server
	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatalf("Failed to gracefully shutdown server: %v", err)
	}
}
