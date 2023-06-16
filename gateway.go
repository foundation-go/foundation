package foundation

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"reflect"
	"runtime"
	"syscall"

	gw_runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	log "github.com/sirupsen/logrus"

	"github.com/ri-nat/foundation/gateway"
)

// StartGatewayOptions represents the options for starting the Foundation gateway.
type StartGatewayOptions struct {
	// Services to register with the gateway
	Services []gateway.Service

	AuthenticationDetailsMiddleware func(http.Handler) http.Handler

	WithAuthentication   bool
	AuthenticationExcept []string

	Middlewares []func(http.Handler) http.Handler
}

func NewStartGatewayOptions() StartGatewayOptions {
	return StartGatewayOptions{}
}

// StartGateway starts the Foundation gateway.
func (app Application) StartGateway(opts StartGatewayOptions) {
	logApplicationStartup("gateway")

	// Start common components
	if err := app.startComponents(); err != nil {
		log.Fatalf("Failed to start components: %v", err)
	}

	mux, err := gateway.RegisterServices(
		opts.Services,
		gw_runtime.WithIncomingHeaderMatcher(gateway.IncomingHeaderMatcher),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	port := GetEnvOrInt("PORT", 51051)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: applyMiddlewares(mux, opts),
	}

	log.Infof("Listening on http://0.0.0.0:%d", port)

	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				log.Println("Server stopped")
			} else {
				log.Fatalf("Failed to start server: %v", err)
			}
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down server...")

	// Gracefully stop the server
	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatalf("Failed to gracefully shutdown server: %v", err)
	}
}

func applyMiddlewares(mux http.Handler, opts StartGatewayOptions) http.Handler {
	log.Info("Using middlewares:")

	// General middlewares
	middlewares := []func(http.Handler) http.Handler{
		gateway.WithRequestLogger,
		gateway.WithCORSEnabled,
	}
	for _, middleware := range middlewares {
		printMiddlewareName(middleware)
		mux = middleware(mux)
	}

	// Authentication details middleware
	if opts.AuthenticationDetailsMiddleware != nil {
		printMiddlewareName(opts.AuthenticationDetailsMiddleware)
		mux = opts.AuthenticationDetailsMiddleware(mux)
	}

	// Authentication middleware
	if opts.WithAuthentication {
		printMiddlewareName(gateway.WithAuthentication)
		mux = gateway.WithAuthentication(mux, opts.AuthenticationExcept)
	}

	// Custom middlewares
	for _, middleware := range opts.Middlewares {
		printMiddlewareName(middleware)
		mux = middleware(mux)
	}

	return mux
}

func printMiddlewareName(middleware interface{}) {
	funcValue := reflect.ValueOf(middleware)
	funcName := runtime.FuncForPC(funcValue.Pointer()).Name()

	log.Infof(" - %s", funcName)
}
