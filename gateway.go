package foundation

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"reflect"
	"runtime"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	gw_runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"github.com/ri-nat/foundation/gateway"
)

const (
	// GatewayDefaultTimeout is the default timeout for downstream services requests.
	GatewayDefaultTimeout = 30 * time.Second
)

// GatewayOptions represents the options for starting the Foundation gateway.
type GatewayOptions struct {
	// Services to register with the gateway
	Services []gateway.Service
	// Timeout for downstream services requests (default: 30 seconds, if constructed with `NewGatewayOptions`)
	Timeout time.Duration
	// AuthenticationDetailsMiddleware is a middleware that populates the request context with authentication details.
	AuthenticationDetailsMiddleware func(http.Handler) http.Handler
	// WithAuthentication enables authentication for the gateway.
	WithAuthentication bool
	// AuthenticationExcept is a list of paths that should not be authenticated.
	AuthenticationExcept []string
	// Middleware is a list of middleware to apply to the gateway. The middleware is applied in the order it is defined.
	Middleware []func(http.Handler) http.Handler
}

// NewGatewayOptions returns a new GatewayOptions with default values.
func NewGatewayOptions() GatewayOptions {
	return GatewayOptions{
		Timeout: GatewayDefaultTimeout,
	}
}

// StartGateway starts the Foundation gateway.
func (s *Service) StartGateway(opts GatewayOptions) {
	s.logStartup("gateway")

	gw_runtime.DefaultContextTimeout = opts.Timeout
	s.Logger.Debugf("Downstream requests timeout: %s", opts.Timeout)

	// Start common components
	if err := s.StartComponents(); err != nil {
		err = fmt.Errorf("failed to start components: %w", err)
		sentry.CaptureException(err)
		// TODO: Maybe flush sentry before exiting via Fatal?
		s.Logger.Fatal(err)
	}

	mux, err := gateway.RegisterServices(
		opts.Services,
		&gateway.RegisterServicesOptions{
			MuxOpts: []gw_runtime.ServeMuxOption{
				gw_runtime.WithIncomingHeaderMatcher(gateway.IncomingHeaderMatcher),
			},
			TLSDir: s.Config.GRPC.TLSDir,
		},
	)
	if err != nil {
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	port := GetEnvOrInt("PORT", 51051)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: s.applyMiddleware(mux, opts),
	}

	s.Logger.Infof("Listening on http://0.0.0.0:%d", port)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			err = fmt.Errorf("failed to start server: %w", err)
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

func (s *Service) applyMiddleware(mux http.Handler, opts GatewayOptions) http.Handler {
	var middleware []func(http.Handler) http.Handler

	// General middleware
	middleware = append(middleware, gateway.WithRequestLogger(s.Logger), gateway.WithCORSEnabled)

	// Authentication details middleware
	if opts.AuthenticationDetailsMiddleware != nil {
		middleware = append(middleware, opts.AuthenticationDetailsMiddleware)
	}

	// Authentication middleware
	if opts.WithAuthentication {
		middleware = append(middleware, gateway.WithAuthentication(opts.AuthenticationExcept))
	}

	// Custom middleware
	middleware = append(middleware, opts.Middleware...)

	// Log middleware chain
	s.logMiddlewareChain(middleware)

	// Apply middleware in reverse order, so the order they are defined is the order they are applied
	for i := len(middleware) - 1; i >= 0; i-- {
		mux = middleware[i](mux)
	}

	return mux
}

func (s *Service) logMiddlewareChain(middleware []func(http.Handler) http.Handler) {
	s.Logger.Info("Using middleware:")

	for _, m := range middleware {
		s.Logger.Infof(" - %s", runtime.FuncForPC(reflect.ValueOf(m).Pointer()).Name())
	}
}
