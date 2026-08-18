package foundation

import (
	"context"
	"errors"
	"net/http"
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

// Start runs the Foundation service in HTTP Server mode.
func (s *HTTPServer) Start(opts *HTTPServerOptions) {
	s.Options = opts

	s.Service.Start(&StartOptions{
		ModeName:               "http_server",
		StartComponentsOptions: s.Options.StartComponentsOptions,
		ServiceFunc:            s.ServiceFunc,
	})
}

func (s *HTTPServer) ServiceFunc(ctx context.Context) error {
	port := GetEnvOrInt("PORT", DefaultPort)
	server := s.newHTTPServer(port, s.Options.Handler)

	s.Logger.Infof("Listening on http://0.0.0.0:%d", port)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.Fatal(err, "failed to start HTTP server")
		}
	}()

	<-ctx.Done()

	// Gracefully stop the HTTP server
	if err := s.shutdownHTTPServer(server); err != nil {
		s.CaptureError(err, "failed to gracefully shutdown HTTP server")
	}

	return nil
}
