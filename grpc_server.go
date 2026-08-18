package foundation

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	fg "github.com/foundation-go/foundation/grpc"
)

// GRPCServer represents a gRPC server mode Foundation service.
type GRPCServer struct {
	*Service

	Options *GRPCServerOptions
}

// InitGRPCServer initializes a new Foundation service in gRPC Server mode.
func InitGRPCServer(name string) *GRPCServer {
	return &GRPCServer{
		Service: Init(name),
	}
}

// GRPCServerOptions are the options to start a Foundation service in gRPC Server mode.
type GRPCServerOptions struct {
	// RegisterFunc is a function that registers the gRPC server implementation.
	RegisterFunc func(s *grpc.Server)

	// GRPCServerOptions are the gRPC server options to use.
	GRPCServerOptions []grpc.ServerOption

	// StartComponentsOptions are the options to start the components.
	StartComponentsOptions []StartComponentsOption
}

func NewGRPCServerOptions() *GRPCServerOptions {
	return &GRPCServerOptions{}
}

// Start initializes the Foundation service in gRPC server mode.
func (s *GRPCServer) Start(opts *GRPCServerOptions) {
	s.Options = opts

	s.Service.Start(&StartOptions{
		ModeName:               "grpc",
		StartComponentsOptions: s.Options.StartComponentsOptions,
		ServiceFunc:            s.ServiceFunc,
	})
}

func (s *GRPCServer) ServiceFunc(ctx context.Context) error {
	loggingOptions := []fg.LoggingOption{fg.WithPayloadLogging(s.Config.LogPayloads)}

	// Default interceptors
	//
	// N.B.: Interceptors are executed in the order they are defined.
	defaultOptions := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(fg.DefaultUnaryInterceptors(s.Logger, loggingOptions...)...),
		grpc.ChainStreamInterceptor(fg.DefaultStreamInterceptors(s.Logger, loggingOptions...)...),
		// Continues the trace the gateway started; without a server handler the
		// incoming traceparent was dropped and every service began a new trace.
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}

	// Prepend the default server options in front of the application-defined ones
	serverOptions := append(defaultOptions, s.Options.GRPCServerOptions...)

	// TLS
	if s.Config.GRPC.TLSDir != "" {
		s.Logger.Debugf("gRPC mTLS is enabled, loading certificates from %s", s.Config.GRPC.TLSDir)

		tlsConfig, err := fg.NewTLSConfig(s.Config.GRPC.TLSDir)
		if err != nil {
			return fmt.Errorf("failed to load TLS config: %w", err)
		}

		serverOptions = append(serverOptions, grpc.Creds(tlsConfig))
	} else if IsProductionEnv() {
		s.Logger.Warn("mTLS for gRPC server is not configured, it is strongly recommended to use mTLS in production")
	}

	// Start the server
	listener := s.acquireListener(ctx)
	server := grpc.NewServer(serverOptions...)

	s.Options.RegisterFunc(server)

	go func() {
		if err := server.Serve(listener); err != nil {
			s.Fatal(err, "failed to serve gRPC")
		}
	}()

	<-ctx.Done()

	s.stopGRPCServer(server)

	return nil
}

// grpcForcedStopGrace is how long a forced Stop is given to unwind before the
// shutdown moves on without it. It is a variable so that tests can shorten it.
var grpcForcedStopGrace = 5 * time.Second

// stopGRPCServer stops a gRPC server gracefully, falling back to a hard stop
// when in-flight calls do not finish within the shutdown budget.
//
// GracefulStop on its own waits forever: a single stuck streaming RPC used to
// keep the process alive until the supervisor killed it.
func (s *Service) stopGRPCServer(server *grpc.Server) {
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)

		server.GracefulStop()
	}()

	timeout := s.shutdownTimeout()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-stopped:
	case <-timer.C:
		s.Logger.Warnf("gRPC server did not stop gracefully within %s; forcing it", timeout)
		server.Stop()

		// Stop closes the listeners and the transports, but it cannot make a
		// handler that ignores its context return — and GracefulStop only
		// returns once every handler has. Waiting unconditionally would hand
		// the rest of the shutdown, Sentry flush included, to a single wedged
		// handler.
		grace := time.NewTimer(grpcForcedStopGrace)
		defer grace.Stop()

		select {
		case <-stopped:
		case <-grace.C:
			s.Logger.Error("gRPC handlers did not return; continuing the shutdown regardless")
		}
	}
}

func (s *Service) acquireListener(ctx context.Context) net.Listener {
	port := GetEnvOrInt("PORT", DefaultPort)

	var config net.ListenConfig

	listener, err := config.Listen(ctx, "tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		s.Fatal(err, fmt.Sprintf("failed to listen on port %d", port))
	}

	s.Logger.Infof("Listening on http://0.0.0.0:%d", port)

	return listener
}
