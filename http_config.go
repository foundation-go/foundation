package foundation

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// contextWithTimeout is a tiny wrapper that keeps the shutdown call sites
// readable.
func contextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

const (
	// DefaultHTTPReadHeaderTimeout bounds how long a client may take to send
	// its request headers.
	//
	// This is the setting that closes Slowloris (gosec G112): without it a
	// handful of connections dribbling headers one byte at a time occupy the
	// server indefinitely. No legitimate client needs seconds to send headers.
	DefaultHTTPReadHeaderTimeout = 10 * time.Second

	// DefaultHTTPIdleTimeout bounds how long a keep-alive connection may sit
	// unused before it is closed.
	DefaultHTTPIdleTimeout = 120 * time.Second

	// DefaultMaxRequestBodySize caps the request body a Foundation HTTP server
	// will read. grpc-gateway unmarshals the whole body into a protobuf
	// message, so an unbounded body is an unbounded allocation.
	DefaultMaxRequestBodySize int64 = 32 << 20 // 32 MiB
)

// HTTPConfig holds the timeouts and limits applied to Foundation's HTTP
// servers: the gateway, the HTTP server mode, and the metrics server.
type HTTPConfig struct {
	// ReadHeaderTimeout bounds reading the request headers. Zero disables it,
	// which reopens Slowloris — leave it set.
	ReadHeaderTimeout time.Duration

	// ReadTimeout bounds reading the entire request, body included. Zero
	// disables it, which is the default: a limit here also caps how long a
	// client may take to upload, and Foundation cannot know what is reasonable
	// for a given service.
	ReadTimeout time.Duration

	// WriteTimeout bounds writing the response. Zero disables it, which is the
	// default: a limit here would cut off long-running responses, including
	// server-streaming gRPC methods exposed through the gateway. Set it to
	// comfortably more than GatewayOptions.Timeout when responses are bounded.
	WriteTimeout time.Duration

	// IdleTimeout bounds how long an idle keep-alive connection is kept.
	IdleTimeout time.Duration

	// MaxRequestBodySize caps the request body in bytes. Zero disables the cap.
	MaxRequestBodySize int64
}

// NewHTTPConfig returns the HTTP configuration populated from the environment.
func NewHTTPConfig() *HTTPConfig {
	return &HTTPConfig{
		ReadHeaderTimeout:  GetEnvOrDuration("HTTP_READ_HEADER_TIMEOUT", DefaultHTTPReadHeaderTimeout),
		ReadTimeout:        GetEnvOrDuration("HTTP_READ_TIMEOUT", 0),
		WriteTimeout:       GetEnvOrDuration("HTTP_WRITE_TIMEOUT", 0),
		IdleTimeout:        GetEnvOrDuration("HTTP_IDLE_TIMEOUT", DefaultHTTPIdleTimeout),
		MaxRequestBodySize: int64(GetEnvOrInt("HTTP_MAX_REQUEST_BODY_SIZE", int(DefaultMaxRequestBodySize))),
	}
}

// httpConfig returns the service's HTTP configuration, falling back to the
// defaults for services constructed without one.
func (s *Service) httpConfig() *HTTPConfig {
	if s.Config != nil && s.Config.HTTP != nil {
		return s.Config.HTTP
	}

	return &HTTPConfig{
		ReadHeaderTimeout:  DefaultHTTPReadHeaderTimeout,
		IdleTimeout:        DefaultHTTPIdleTimeout,
		MaxRequestBodySize: DefaultMaxRequestBodySize,
	}
}

// newHTTPServer builds an http.Server with the service's timeouts and body
// limit applied.
func (s *Service) newHTTPServer(port int, handler http.Handler) *http.Server {
	cfg := s.httpConfig()

	if cfg.MaxRequestBodySize > 0 {
		handler = http.MaxBytesHandler(handler, cfg.MaxRequestBodySize)
	}

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

// shutdownHTTPServer gracefully stops an HTTP server within the service's
// shutdown budget.
//
// The previous `server.Shutdown(context.Background())` had no deadline at all,
// so a single hung handler kept the process alive until the supervisor killed
// it — taking the rest of the shutdown sequence with it.
func (s *Service) shutdownHTTPServer(server *http.Server) error {
	ctx, cancel := contextWithTimeout(s.shutdownTimeout())
	defer cancel()

	return server.Shutdown(ctx)
}
