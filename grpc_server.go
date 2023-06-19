package foundation

import (
	"context"
	"fmt"
	"net"
	"os/signal"
	"syscall"

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
		app.Logger.Fatalf("Failed to start components: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	listener := app.aquireListener()
	server := grpc.NewServer(opts.GRPCServerOptions...)

	opts.RegisterFunc(server)

	go func() {
		if err := server.Serve(listener); err != nil {
			app.Logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	<-ctx.Done()
	app.Logger.Println("Shutting down server...")

	// Gracefully stop the server
	server.GracefulStop()
	app.StopComponents()

	app.Logger.Println("Server gracefully stopped")
}

func (app *Application) aquireListener() net.Listener {
	port := GetEnvOrInt("PORT", 51051)
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		app.Logger.Fatalf("Failed to listen port %d: %v", port, err)
	}

	app.Logger.Infof("Listening on http://0.0.0.0:%d", port)

	return listener
}
