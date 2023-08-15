package foundation

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/getsentry/sentry-go"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	cable_grpc "github.com/ri-nat/foundation/cable/grpc"
	pb "github.com/ri-nat/foundation/cable/grpc/proto"
	fg "github.com/ri-nat/foundation/grpc"
	"google.golang.org/grpc"
)

// CableGRPCOptions are the options to start a Foundation service in gRPC Server mode.
type CableGRPCOptions struct {
	// GRPCServerOptions are the gRPC server options to use.
	GRPCServerOptions []grpc.ServerOption

	// StartComponentsOptions are the options to start the components.
	StartComponentsOptions []StartComponentsOption

	// Channels are the channels to use.
	Channels map[string]cable_grpc.Channel

	// WithAuthentication enables authentication.
	WithAuthentication bool
	// AuthenticationFunc is the function to use for authentication.
	AuthenticationFunc cable_grpc.AuthenticationFunc
}

func NewCableGRPCOptions() CableGRPCOptions {
	return CableGRPCOptions{}
}

// StartCableGRPC starts a Foundation as an AnyCable-compartible gRPC server.
func (s *Service) StartCableGRPC(opts CableGRPCOptions) {
	s.logStartup("cable_grpc")

	// Start common components
	if err := s.StartComponents(opts.StartComponentsOptions...); err != nil {
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
		fg.LoggingInterceptor(s.Logger),
	}
	chainedInterceptor := grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(interceptors...))
	opts.GRPCServerOptions = append(opts.GRPCServerOptions, chainedInterceptor)

	// Start the server
	listener := s.aquireListener()
	server := grpc.NewServer(opts.GRPCServerOptions...)

	pb.RegisterRPCServer(server, &cable_grpc.Server{
		Channels:           opts.Channels,
		WithAuthentication: opts.WithAuthentication,
		AuthenticationFunc: opts.AuthenticationFunc,
		Logger:             s.Logger,
	})

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
