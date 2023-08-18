package foundation

import (
	"context"
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"
)

// HTTPServer represents a HTTP Server mode Foundation service.
type HTTPServer struct {
	*Service

	Options *HTTPServerOptions
}

// InitHTTPServer initializes a new Foundation service in HTTP Server mode.
func InitHTTPServer(name string) *HTTPServer {
	return &HTTPServer{
		Init(name),
		NewHTTPServerOptions(),
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
	s.Options = opts

	s.Service.Start(&StartOptions{
		ModeName:               "http_server",
		StartComponentsOptions: s.Options.StartComponentsOptions,
		ServiceFunc:            s.ServiceFunc,
	})
}

func (s *HTTPServer) ServiceFunc(ctx context.Context) error {
	port := GetEnvOrInt("PORT", 51051)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: s.Options.Handler,
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

	// Gracefully stop the HTTP server
	if err := server.Shutdown(context.Background()); err != nil {
		err = fmt.Errorf("failed to gracefully shutdown HTTP server: %w", err)
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	return nil
}
