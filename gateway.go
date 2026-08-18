package foundation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"time"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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
	// MuxOpts are additional grpc-gateway ServeMux options.
	//
	// Precedence:
	//  1) Foundation applies its defaults first (incoming/outgoing header matchers, and the default marshaler option).
	//  2) Then MuxOpts are applied in the order provided.
	//
	// This means MuxOpts can override Foundation defaults (e.g. add a later WithMarshalerOption).
	MuxOpts []gwruntime.ServeMuxOption
	// AuthenticationDetailsMiddleware is a middleware that populates the request context with authentication details.
	//
	// It is the only place in the chain that runs before authentication is
	// enforced, so it is required whenever WithAuthentication is enabled.
	AuthenticationDetailsMiddleware func(http.Handler) http.Handler
	// WithAuthentication enables authentication for the gateway.
	WithAuthentication bool
	// AuthenticationExcept is a list of paths that should not be authenticated.
	AuthenticationExcept []string
	// TrustInboundAuthenticationHeaders disables stripping the identity-bearing
	// Foundation headers (`X-Authenticated`, `X-User-Id`, `X-Client-Id`,
	// `X-Scope`, `X-Metadata`) from incoming requests.
	//
	// Leave this off unless the gateway is only reachable through a trusted
	// proxy that sets those headers itself. With it on, any client that can
	// reach the gateway can impersonate any user.
	TrustInboundAuthenticationHeaders bool
	// Middleware is a list of middleware to apply to the gateway. The middleware is applied in the order it is defined.
	Middleware []func(http.Handler) http.Handler
	// StartComponentsOptions are the options to start the components.
	StartComponentsOptions []StartComponentsOption
	// CORSOptions are the options for CORS.
	CORSOptions *gateway.CORSOptions
	// MarshalOptions are used only for the default JSONPb marshaler when Marshaler is nil.
	MarshalOptions protojson.MarshalOptions
	// Marshaler overrides the default marshaler used by grpc-gateway.
	//
	// If nil, Foundation uses JSONPb configured with MarshalOptions.
	// If non-nil, MarshalOptions is not applied automatically; include it in your custom marshaler if needed.
	//
	// Note: MuxOpts are applied after the default marshaler option, so a later
	// runtime.WithMarshalerOption(...) in MuxOpts can override this as well.
	Marshaler gwruntime.Marshaler
	// SwaggerEndpoints is a list of endpoints to serve swagger JSON files.
	SwaggerEndpoints []gateway.SwaggerEndpoint
}

// NewGatewayOptions returns a new GatewayOptions with default values.
func NewGatewayOptions() *GatewayOptions {
	return &GatewayOptions{
		Timeout:     GatewayDefaultTimeout,
		CORSOptions: gateway.NewCORSOptions(),
	}
}

// Validate reports configuration mistakes that would leave the gateway in an
// unsafe or non-functional state.
func (o *GatewayOptions) Validate() error {
	// With TrustInboundAuthenticationHeaders the identity headers come from the
	// proxy in front of the gateway, so there is nothing for a details
	// middleware to do — that deployment is exactly what the option exists for.
	if o.WithAuthentication && o.AuthenticationDetailsMiddleware == nil && !o.TrustInboundAuthenticationHeaders {
		return errors.New(
			"WithAuthentication is enabled but AuthenticationDetailsMiddleware is not set: " +
				"without it nothing populates the identity headers, and WithAuthentication " +
				"would accept whatever the client sent. Set AuthenticationDetailsMiddleware " +
				"(e.g. gateway.WithHydraAuthenticationDetails), or set " +
				"TrustInboundAuthenticationHeaders if a trusted proxy in front of this " +
				"gateway already sets them, or disable WithAuthentication",
		)
	}

	return nil
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
	if err := s.Options.Validate(); err != nil {
		return fmt.Errorf("invalid gateway options: %w", err)
	}

	gwruntime.DefaultContextTimeout = s.Options.Timeout
	s.Logger.Debugf("Downstream requests timeout: %s", s.Options.Timeout)

	marshaler := s.Options.Marshaler
	if marshaler == nil {
		marshaler = &gwruntime.JSONPb{
			MarshalOptions: s.Options.MarshalOptions,
		}
	}

	muxOpts := []gwruntime.ServeMuxOption{
		gwruntime.WithIncomingHeaderMatcher(gateway.IncomingHeaderMatcher),
		gwruntime.WithOutgoingHeaderMatcher(gateway.OutgoingHeaderMatcher),
		gwruntime.WithMarshalerOption(gwruntime.MIMEWildcard, marshaler),
		// Lets the metrics middleware label by matched route instead of by
		// request path, which would be unbounded cardinality.
		gwruntime.WithMetadata(gateway.RouteAnnotator),
	}
	muxOpts = append(muxOpts, s.Options.MuxOpts...)

	// N.B.: context.WithoutCancel, deliberately. grpc-gateway's generated
	// registration closes each downstream connection when this context is
	// cancelled — and `ctx` is cancelled at the *start* of the shutdown, which
	// would kill every in-flight proxied call with "grpc: the client
	// connection is closing" while the HTTP server is still politely draining
	// them. The deferred mux.Close below releases the connections after the
	// drain, which is the right moment.
	mux, err := gateway.RegisterServices(
		context.WithoutCancel(ctx),
		s.Options.Services,
		&gateway.RegisterServicesOptions{
			MuxOpts: muxOpts,
			TLSDir:  s.Config.GRPC.TLSDir,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register services: %w", err)
	}
	defer mux.Close() //nolint:errcheck // best effort on shutdown

	port := GetEnvOrInt("PORT", DefaultPort)

	// otelhttp wraps the whole chain so that the server span is the parent of
	// everything the request does, including the client spans for downstream
	// gRPC calls, which previously had no parent at all.
	handler := otelhttp.NewHandler(
		s.applyMiddleware(mux, s.Options),
		"gateway",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string { return r.Method }),
	)

	server := s.newHTTPServer(port, handler)

	s.Logger.Infof("Listening on http://0.0.0.0:%d", port)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.Fatal(err, "failed to start gateway HTTP server")
		}
	}()

	<-ctx.Done()

	// Gracefully stop the HTTP server.
	//
	// A shutdown that runs out of budget is reported, not returned: returning
	// it reaches Service.Fatal, which exits(1) and skips StopComponents — so a
	// single slow request would cost the Kafka flush, the Sentry flush and a
	// clean exit code on an otherwise ordinary rollout.
	if err := s.shutdownHTTPServer(server); err != nil {
		s.CaptureError(err, "failed to gracefully shutdown the gateway HTTP server")
	}

	return nil
}

func (s *Service) applyMiddleware(mux http.Handler, opts *GatewayOptions) http.Handler {
	var middleware []func(http.Handler) http.Handler

	// Trust boundary: drop identity headers supplied by the client before
	// anything downstream — including WithAuthentication — gets to read them.
	// This has to stay first in the chain.
	if !opts.TrustInboundAuthenticationHeaders {
		middleware = append(middleware, gateway.StripClientAuthenticationHeaders)
	} else {
		s.Logger.Warn(
			"TrustInboundAuthenticationHeaders is enabled: identity headers are taken from the " +
				"client as-is. Only do this behind a trusted proxy.",
		)
	}

	// General middleware.
	//
	// Recovery sits just inside the request logger, so a panic report carries
	// the request's fields, and outside everything else.
	middleware = append(middleware,
		gateway.WithMetrics,
		gateway.WithRequestLogger(s.Logger),
		gateway.WithRecovery,
		gateway.WithCORSEnabled(opts.CORSOptions),
	)

	// Swagger middleware
	if len(opts.SwaggerEndpoints) > 0 {
		swaggerMiddleware, err := gateway.WithSwagger(opts.SwaggerEndpoints)
		if err != nil {
			s.Logger.Warnf("Failed to load swagger file: %v", err)
		} else {
			middleware = append(middleware, swaggerMiddleware)
		}
	}

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
