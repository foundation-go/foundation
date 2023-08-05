package foundation

import (
	"context"
	"fmt"
	"net"
	"os/signal"
	"syscall"

	"github.com/getsentry/sentry-go"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	fg "github.com/ri-nat/foundation/grpc"
	"google.golang.org/grpc"
)

// GRPCServerOptions are the options to start a Foundation service in gRPC Server mode.
type GRPCServerOptions struct {
	// RegisterFunc is a function that registers the gRPC server implementation.
	RegisterFunc func(s *grpc.Server)

	// GRPCServerOptions are the gRPC server options to use.
	GRPCServerOptions []grpc.ServerOption
}

func NewGRPCServerOptions() GRPCServerOptions {
	return GRPCServerOptions{}
}

// StartGRPCServer starts a Foundation service in gRPC server mode.
func (s *Service) StartGRPCServer(opts GRPCServerOptions) {
	s.logStartup("grpc")

	// Start common components
	if err := s.StartComponents(); err != nil {
		err = fmt.Errorf("failed to start components: %w", err)
		sentry.CaptureException(err)
		s.Logger.Fatalf("Failed to start components: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Default interceptors
	//
	// TODO: Work correctly with interceptors from options
	interceptors := []grpc.UnaryServerInterceptor{
		fg.MetadataInterceptor,
		fg.LoggingInterceptor(s.Logger),
		s.foundationErrorToStatusInterceptor,
	}
	chainedInterceptor := grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(interceptors...))
	opts.GRPCServerOptions = append(opts.GRPCServerOptions, chainedInterceptor)

	// TLS
	if s.Config.GRPC.TLSDir != "" {
		s.Logger.Debugf("gRPC mTLS is enabled, loading certificates from %s", s.Config.GRPC.TLSDir)

		tlsConfig, err := fg.NewTLSConfig(s.Config.GRPC.TLSDir)
		if err != nil {
			s.Logger.Fatalf("Failed to configure TLS: %v", err)
		}

		opts.GRPCServerOptions = append(opts.GRPCServerOptions, grpc.Creds(tlsConfig))
	} else if IsProductionEnv() {
		s.Logger.Warn("mTLS for gRPC server is not configured, it is strongly recommended to use mTLS in production")
	}

	// Start the server
	listener := s.aquireListener()
	server := grpc.NewServer(opts.GRPCServerOptions...)

	opts.RegisterFunc(server)

	go func() {
		if err := server.Serve(listener); err != nil {
			err = fmt.Errorf("failed to start server: %w", err)
			sentry.CaptureException(err)
			s.Logger.Fatal(err)
		}
	}()

	<-ctx.Done()
	s.Logger.Println("Shutting down service...")

	// Gracefully stop the server
	server.GracefulStop()
	s.StopComponents()

	s.Logger.Println("Service gracefully stopped")
}

func (s *Service) aquireListener() net.Listener {
	port := GetEnvOrInt("PORT", 51051)
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		err = fmt.Errorf("failed to listen port %d: %w", port, err)
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	s.Logger.Infof("Listening on http://0.0.0.0:%d", port)

	return listener
}
