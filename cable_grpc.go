package foundation

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	cablegrpc "github.com/foundation-go/foundation/cable/grpc"
	pb "github.com/foundation-go/foundation/cable/grpc/proto"
	fg "github.com/foundation-go/foundation/grpc"
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
	// N.B.: Interceptors are executed in the order they are defined.
	defaultOptions := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			fg.RecoveryUnaryInterceptor(s.Logger),
			fg.MetricsUnaryInterceptor,
			cablegrpc.LoggingUnaryInterceptor(s.Logger),
		),
		grpc.ChainStreamInterceptor(fg.RecoveryStreamInterceptor(s.Logger)),
	}

	// mTLS, on the same terms as the regular gRPC mode. The cable server speaks
	// to AnyCable across the cluster and carries user identity in both
	// directions; it had no transport security option at all.
	if s.Config.GRPC.TLSDir != "" {
		s.Logger.Debugf("Cable gRPC mTLS is enabled, loading certificates from %s", s.Config.GRPC.TLSDir)

		tlsConfig, err := fg.NewTLSConfig(s.Config.GRPC.TLSDir)
		if err != nil {
			return fmt.Errorf("failed to load TLS config: %w", err)
		}

		defaultOptions = append(defaultOptions, grpc.Creds(tlsConfig))
	} else if IsProductionEnv() {
		s.Logger.Warn("mTLS for the cable gRPC server is not configured; it is strongly recommended in production")
	}

	// Prepend the default server options in front of the application-defined ones
	serverOptions := append(defaultOptions, s.Options.GRPCServerOptions...)

	// Start the server
	listener := s.acquireListener(ctx)
	server := grpc.NewServer(serverOptions...)

	pb.RegisterRPCServer(server, &cablegrpc.Server{
		Channels:           s.Options.Channels,
		WithAuthentication: s.Options.WithAuthentication,
		AuthenticationFunc: s.Options.AuthenticationFunc,
		Logger:             s.Logger,
	})

	go func() {
		if err := server.Serve(listener); err != nil {
			s.Fatal(err, "failed to serve cable gRPC")
		}
	}()

	<-ctx.Done()

	s.stopGRPCServer(server)

	return nil
}
