package foundation

import (
	"context"
	"fmt"

	cablegrpc "github.com/foundation-go/foundation/cable/grpc"
	pb "github.com/foundation-go/foundation/cable/grpc/proto"
	fg "github.com/foundation-go/foundation/grpc"
	"github.com/getsentry/sentry-go"
	grpcmiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
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
	Channels map[string]cablegrpc.Channel

	// WithAuthentication enables authentication.
	WithAuthentication bool
	// AuthenticationFunc is the function to use for authentication.
	AuthenticationFunc cablegrpc.AuthenticationFunc
}

func NewCableGRPCOptions() *CableGRPCOptions {
	return &CableGRPCOptions{}
}

// Start runs the Foundation as an AnyCable-compartible gRPC server.
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
	chainedInterceptor := grpc.UnaryInterceptor(grpcmiddleware.ChainUnaryServer(interceptors...))
	s.Options.GRPCServerOptions = append(s.Options.GRPCServerOptions, chainedInterceptor)

	// Start the server
	listener := s.acquireListener()
	server := grpc.NewServer(s.Options.GRPCServerOptions...)

	pb.RegisterRPCServer(server, &cablegrpc.Server{
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
