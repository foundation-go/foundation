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

// StartGRPCServerOptions are the options to start a Foundation application in gRPC Server mode.
type StartGRPCServerOptions struct {
	// RegisterFunc is a function that registers the gRPC server implementation.
	RegisterFunc func(s *grpc.Server)

	// GRPCServerOptions are the gRPC server options to use.
	GRPCServerOptions []grpc.ServerOption
}

func NewStartGRPCServerOptions() StartGRPCServerOptions {
	return StartGRPCServerOptions{}
}

// StartGRPCServer starts a Foundation application in gRPC server mode.
func (app *Application) StartGRPCServer(opts StartGRPCServerOptions) {
	app.logStartup("grpc")

	// Start common components
	if err := app.StartComponents(); err != nil {
		err = fmt.Errorf("failed to start components: %w", err)
		sentry.CaptureException(err)
		app.Logger.Fatalf("Failed to start components: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Default interceptors
	//
	// TODO: Work correctly with interceptors from options
	interceptors := []grpc.UnaryServerInterceptor{
		fg.MetadataInterceptor,
		fg.LoggingInterceptor(app.Logger),
		app.foundationErrorToStatusInterceptor,
	}
	chainedInterceptor := grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(interceptors...))
	opts.GRPCServerOptions = append(opts.GRPCServerOptions, chainedInterceptor)

	// Start the server
	listener := app.aquireListener()
	server := grpc.NewServer(opts.GRPCServerOptions...)

	opts.RegisterFunc(server)

	go func() {
		if err := server.Serve(listener); err != nil {
			err = fmt.Errorf("failed to start server: %w", err)
			sentry.CaptureException(err)
			app.Logger.Fatal(err)
		}
	}()

	<-ctx.Done()
	app.Logger.Println("Shutting down server...")

	// Gracefully stop the server
	server.GracefulStop()
	app.StopComponents()

	app.Logger.Println("Application gracefully stopped")
}

func (app *Application) aquireListener() net.Listener {
	port := GetEnvOrInt("PORT", 51051)
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		err = fmt.Errorf("failed to listen port %d: %w", port, err)
		sentry.CaptureException(err)
		app.Logger.Fatal(err)
	}

	app.Logger.Infof("Listening on http://0.0.0.0:%d", port)

	return listener
}
