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

	"github.com/ri-nat/foundation/gateway"
)

// StartGatewayOptions represents the options for starting the Foundation gateway.
type StartGatewayOptions struct {
	// Services to register with the gateway
	Services []gateway.Service

	AuthenticationDetailsMiddleware func(http.Handler) http.Handler

	WithAuthentication   bool
	AuthenticationExcept []string

	Middleware []func(http.Handler) http.Handler
}

func NewStartGatewayOptions() StartGatewayOptions {
	return StartGatewayOptions{}
}

// StartGateway starts the Foundation gateway.
func (app *Application) StartGateway(opts StartGatewayOptions) {
	app.logStartup("gateway")

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
	app.Logger.Info("Using middleware (in reverse order):")

	// Custom middleware
	for i := len(opts.Middleware) - 1; i >= 0; i-- {
		m := opts.Middleware[i]
		app.printMiddlewareName(m)
		mux = m(mux)
	}

	// Authentication middleware
	if opts.WithAuthentication {
		app.printMiddlewareName(gateway.WithAuthentication)
		mux = gateway.WithAuthentication(mux, opts.AuthenticationExcept)
	}

	// Authentication details middleware
	if opts.AuthenticationDetailsMiddleware != nil {
		app.printMiddlewareName(opts.AuthenticationDetailsMiddleware)
		mux = opts.AuthenticationDetailsMiddleware(mux)
	}

	// General middleware
	middleware := []func(http.Handler) http.Handler{
		gateway.WithRequestLogger(app.Logger),
		gateway.WithCORSEnabled,
	}
	for i := len(middleware) - 1; i >= 0; i-- {
		m := middleware[i]
		app.printMiddlewareName(m)
		mux = m(mux)
	}

	return mux
}

func (app *Application) printMiddlewareName(middleware interface{}) {
	funcValue := reflect.ValueOf(middleware)
	funcName := runtime.FuncForPC(funcValue.Pointer()).Name()

	app.Logger.Infof(" - %s", funcName)
}
