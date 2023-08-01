package foundation

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"reflect"
	"runtime"
	"syscall"
	"time"

	gw_runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"github.com/ri-nat/foundation/gateway"
)

const (
	// GatewayDefaultTimeout is the default timeout for downstream services requests.
	GatewayDefaultTimeout = 30 * time.Second
)

// StartGatewayOptions represents the options for starting the Foundation gateway.
type StartGatewayOptions struct {
	// Services to register with the gateway
	Services []gateway.Service
	// Timeout for downstream services requests
	Timeout time.Duration

	AuthenticationDetailsMiddleware func(http.Handler) http.Handler

	WithAuthentication   bool
	AuthenticationExcept []string

	Middleware []func(http.Handler) http.Handler
}

func NewStartGatewayOptions() StartGatewayOptions {
	return StartGatewayOptions{
		Timeout: GatewayDefaultTimeout,
	}
}

// StartGateway starts the Foundation gateway.
func (app *Application) StartGateway(opts StartGatewayOptions) {
	app.logStartup("gateway")

	gw_runtime.DefaultContextTimeout = opts.Timeout
	app.Logger.Debugf("Downstream request timeout: %s", opts.Timeout)

	// Start common components
	if err := app.StartComponents(); err != nil {
		app.Logger.Fatalf("Failed to start components: %v", err)
	}

	mux, err := gateway.RegisterServices(
		opts.Services,
		gw_runtime.WithIncomingHeaderMatcher(gateway.IncomingHeaderMatcher),
	)
	if err != nil {
		app.Logger.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	port := GetEnvOrInt("PORT", 51051)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: app.applyMiddleware(mux, opts),
	}

	app.Logger.Infof("Listening on http://0.0.0.0:%d", port)

	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				app.Logger.Println("Server stopped")
			} else {
				app.Logger.Fatalf("Failed to start server: %v", err)
			}
		}
	}()

	<-ctx.Done()
	app.Logger.Println("Shutting down server...")

	// Gracefully stop the server
	if err := server.Shutdown(context.Background()); err != nil {
		app.Logger.Fatalf("Failed to gracefully shutdown server: %v", err)
	}

	app.StopComponents()

	app.Logger.Println("Server gracefully stopped")
}

func (app *Application) applyMiddleware(mux http.Handler, opts StartGatewayOptions) http.Handler {
	var middleware []func(http.Handler) http.Handler

	// General middleware
	middleware = append(middleware, gateway.WithRequestLogger(app.Logger), gateway.WithCORSEnabled)

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
	app.logMiddlewareChain(middleware)

	// Apply middleware in reverse order, so the order they are defined is the order they are applied
	for i := len(middleware) - 1; i >= 0; i-- {
		mux = middleware[i](mux)
	}

	return mux
}

func (app *Application) logMiddlewareChain(middleware []func(http.Handler) http.Handler) {
	app.Logger.Info("Using middleware:")

	for _, m := range middleware {
		app.Logger.Infof(" - %s", runtime.FuncForPC(reflect.ValueOf(m).Pointer()).Name())
	}
}
