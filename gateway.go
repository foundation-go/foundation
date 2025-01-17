package foundation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"time"

	"github.com/getsentry/sentry-go"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/foundation-go/foundation/gateway"
)

const (
	// GatewayDefaultTimeout is the default timeout for downstream services requests.
	GatewayDefaultTimeout = 30 * time.Second
)

// Gateway represents a gateway mode Foundation service.
type Gateway struct {
	*Service

	Options *GatewayOptions
}

// InitGateway initializes a new Foundation service in Gateway mode.
func InitGateway(name string) *Gateway {
	return &Gateway{
		Service: Init(name),
	}
}

// GatewayOptions represents the options for starting the Foundation gateway.
type GatewayOptions struct {
	// Services to register with the gateway
	Services []*gateway.Service
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
	// StartComponentsOptions are the options to start the components.
	StartComponentsOptions []StartComponentsOption
	// CORSOptions are the options for CORS.
	CORSOptions *gateway.CORSOptions
	// MarshalOptions are the options for the JSONPb marshaler.
	MarshalOptions protojson.MarshalOptions
}

// NewGatewayOptions returns a new GatewayOptions with default values.
func NewGatewayOptions() *GatewayOptions {
	return &GatewayOptions{
		Timeout:     GatewayDefaultTimeout,
		CORSOptions: gateway.NewCORSOptions(),
	}
}

// Start runs the Foundation gateway.
func (s *Gateway) Start(opts *GatewayOptions) {
	s.Options = opts

	s.Service.Start(&StartOptions{
		ModeName:               "gateway",
		StartComponentsOptions: s.Options.StartComponentsOptions,
		ServiceFunc:            s.ServiceFunc,
	})
}

func (s *Gateway) ServiceFunc(ctx context.Context) error {
	gwruntime.DefaultContextTimeout = s.Options.Timeout
	s.Logger.Debugf("Downstream requests timeout: %s", s.Options.Timeout)

	tracingShutdown := s.initTracing()
	defer tracingShutdown()

	mux, err := gateway.RegisterServices(
		s.Options.Services,
		&gateway.RegisterServicesOptions{
			MuxOpts: []gwruntime.ServeMuxOption{
				gwruntime.WithIncomingHeaderMatcher(gateway.IncomingHeaderMatcher),
				gwruntime.WithOutgoingHeaderMatcher(gateway.OutgoingHeaderMatcher),
				gwruntime.WithMarshalerOption(gwruntime.MIMEWildcard, &gwruntime.JSONPb{
					MarshalOptions: s.Options.MarshalOptions,
				}),
			},
			TLSDir: s.Config.GRPC.TLSDir,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register services: %w", err)
	}

	port := GetEnvOrInt("PORT", 51051)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: s.applyMiddleware(mux, s.Options),
	}

	s.Logger.Infof("Listening on http://0.0.0.0:%d", port)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			err = fmt.Errorf("failed to start server: %w", err)
			sentry.CaptureException(err)
			s.Logger.Fatal(err)
		}
	}()

	<-ctx.Done()

	// Gracefully stop the HTTP server
	if err := server.Shutdown(context.Background()); err != nil {
		err = fmt.Errorf("failed to gracefully shutdown HTTP server: %w", err)
		return err
	}

	return nil
}

func (s *Service) applyMiddleware(mux http.Handler, opts *GatewayOptions) http.Handler {
	var middleware []func(http.Handler) http.Handler

	// General middleware
	middleware = append(middleware, gateway.WithRequestLogger(s.Logger), gateway.WithCORSEnabled(opts.CORSOptions))

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
