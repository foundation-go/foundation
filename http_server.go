package foundation

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/getsentry/sentry-go"
)

// HTTPServer represents a HTTP Server mode Foundation service.
type HTTPServer struct {
	*Service
}

// InitHTTPServer initializes a new Foundation service in HTTP Server mode.
func InitHTTPServer(name string) *HTTPServer {
	if name == "" {
		name = "http_server"
	}

	return &HTTPServer{
		Init(name),
	}
}

// HTTPServerOptions are the options to start a Foundation service in HTTP Server mode.
type HTTPServerOptions struct {
	// Handler is the HTTP handler to use.
	Handler http.Handler

	// StartComponentsOptions are the options to start the components.
	StartComponentsOptions []StartComponentsOption
}

func NewHTTPServerOptions() *HTTPServerOptions {
	return &HTTPServerOptions{}
}

// Start starts a Foundation service in HTTP Server mode.
func (s *HTTPServer) Start(opts *HTTPServerOptions) {
	s.logStartup("http")

	// Start common components
	if err := s.StartComponents(opts.StartComponentsOptions...); err != nil {
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
