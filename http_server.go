package foundation

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/getsentry/sentry-go"
)

// HTTPServerOptions are the options to start a Foundation service in server mode.
type HTTPServerOptions struct {
	// Handler is the HTTP handler to use.
	Handler http.Handler
}

func NewHTTPServerOptions() HTTPServerOptions {
	return HTTPServerOptions{}
}

// StartHTTPServer starts a Foundation service in HTTP Server mode.
func (s *Service) StartHTTPServer(opts HTTPServerOptions) {
	s.logStartup("http")

	// Start common components
	if err := s.StartComponents(); err != nil {
		err = fmt.Errorf("failed to start components: %w", err)
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	port := GetEnvOrInt("PORT", 51051)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: opts.Handler,
	}

	s.Logger.Infof("Listening on http://0.0.0.0:%d", port)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			err = fmt.Errorf("failed to start HTTP server: %w", err)
			sentry.CaptureException(err)
			s.Logger.Fatal(err)
		}
	}()

	<-ctx.Done()
	s.Logger.Println("Shutting down service...")

	// Gracefully stop the HTTP server
	if err := server.Shutdown(context.Background()); err != nil {
		err = fmt.Errorf("failed to gracefully shutdown HTTP server: %w", err)
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	s.StopComponents()

	s.Logger.Println("Service gracefully stopped")
}
