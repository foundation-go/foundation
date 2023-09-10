package foundation

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	cable_grpc "github.com/ri-nat/foundation/cable/grpc"
	pb "github.com/ri-nat/foundation/cable/grpc/proto"
	fg "github.com/ri-nat/foundation/grpc"
	"google.golang.org/grpc"
)

// CableGRPC is a Foundation service in AnyCable gRPC Server mode.
type CableGRPC struct {
	*Service

	Options *CableGRPCOptions
}

// InitCableGRPC initializes a Foundation service in AnyCable gRPC Server mode.
func InitCableGRPC(name string) *CableGRPC {
	return &CableGRPC{
		Service: Init(name),
	}
}

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

func NewCableGRPCOptions() *CableGRPCOptions {
	return &CableGRPCOptions{}
}

// Start starts a Foundation as an AnyCable-compartible gRPC server.
func (s *CableGRPC) Start(opts *CableGRPCOptions) {
	s.Options = opts

	startOpts := &StartOptions{
		ModeName:               "cable_grpc",
		StartComponentsOptions: s.Options.StartComponentsOptions,
		ServiceFunc:            s.ServiceFunc,
	}

	s.Service.Start(startOpts)
}

func (s *CableGRPC) ServiceFunc(ctx context.Context) error {
	// Default interceptors
	//
	// TODO: Work correctly with interceptors from s.Options
	interceptors := []grpc.UnaryServerInterceptor{
		fg.LoggingUnaryInterceptor(s.Logger),
	}
	chainedInterceptor := grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(interceptors...))
	s.Options.GRPCServerOptions = append(s.Options.GRPCServerOptions, chainedInterceptor)

	// Start the server
	listener := s.acquireListener()
	server := grpc.NewServer(s.Options.GRPCServerOptions...)

	pb.RegisterRPCServer(server, &cable_grpc.Server{
		Channels:           s.Options.Channels,
		WithAuthentication: s.Options.WithAuthentication,
		AuthenticationFunc: s.Options.AuthenticationFunc,
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
	server.GracefulStop()

	return nil
}
