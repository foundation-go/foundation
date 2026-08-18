package foundation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	fsentry "github.com/foundation-go/foundation/sentry"
)

const (
	MetricsServerComponentName = "metrics-server"

	// MetricsServerDefaultPort is the port the metrics and probe endpoints are
	// served on.
	MetricsServerDefaultPort = 51077

	// metricsServerShutdownTimeout bounds the graceful shutdown of the server.
	metricsServerShutdownTimeout = 5 * time.Second
)

type MetricsServerComponent struct {
	healthHandler    http.HandlerFunc
	livenessHandler  http.HandlerFunc
	readinessHandler http.HandlerFunc
	logger           *logrus.Entry
	port             int
	httpConfig       *HTTPConfig
	server           *http.Server
}

type MetricsServerComponentOption func(*MetricsServerComponent)

// WithMetricsServerLogger sets the logger for the MetricsServer component.
func WithMetricsServerLogger(logger *logrus.Entry) MetricsServerComponentOption {
	return func(c *MetricsServerComponent) {
		if logger == nil {
			return
		}

		c.logger = logger.WithField("component", c.Name())
	}
}

// WithMetricsServerPort sets the port for the MetricsServer component.
func WithMetricsServerPort(port int) MetricsServerComponentOption {
	return func(c *MetricsServerComponent) {
		c.port = port
	}
}

// WithMetricsServerHealthHandler sets the handler for the legacy `/health`
// endpoint, which has readiness semantics.
func WithMetricsServerHealthHandler(handler http.HandlerFunc) MetricsServerComponentOption {
	return func(c *MetricsServerComponent) {
		c.healthHandler = handler
	}
}

// WithMetricsServerLivenessHandler sets the handler for `/live`.
func WithMetricsServerLivenessHandler(handler http.HandlerFunc) MetricsServerComponentOption {
	return func(c *MetricsServerComponent) {
		c.livenessHandler = handler
	}
}

// WithMetricsServerHTTPConfig applies the service's HTTP timeouts to the
// metrics server.
//
// ENV.md said the HTTP_* settings covered this server; they did not, so an
// operator tightening them after a scan flagged the probe port changed nothing
// there.
func WithMetricsServerHTTPConfig(config *HTTPConfig) MetricsServerComponentOption {
	return func(c *MetricsServerComponent) {
		if config != nil {
			c.httpConfig = config
		}
	}
}

// WithMetricsServerReadinessHandler sets the handler for `/ready`.
func WithMetricsServerReadinessHandler(handler http.HandlerFunc) MetricsServerComponentOption {
	return func(c *MetricsServerComponent) {
		c.readinessHandler = handler
	}
}

func NewMetricsServerComponent(opts ...MetricsServerComponentOption) *MetricsServerComponent {
	c := &MetricsServerComponent{
		port:   MetricsServerDefaultPort,
		logger: logrus.NewEntry(logrus.StandardLogger()).WithField("component", MetricsServerComponentName),
		httpConfig: &HTTPConfig{
			ReadHeaderTimeout: DefaultHTTPReadHeaderTimeout,
			IdleTimeout:       DefaultHTTPIdleTimeout,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	mux := http.NewServeMux()

	// http.ServeMux panics on a nil handler, so only register what was given.
	if c.healthHandler != nil {
		mux.HandleFunc("/health", c.healthHandler)
	}
	if c.livenessHandler != nil {
		mux.HandleFunc("/live", c.livenessHandler)
	}
	if c.readinessHandler != nil {
		mux.HandleFunc("/ready", c.readinessHandler)
	}

	mux.Handle("/metrics", promhttp.Handler())

	c.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", c.port),
		Handler:           mux,
		ReadHeaderTimeout: c.httpConfig.ReadHeaderTimeout,
		ReadTimeout:       c.httpConfig.ReadTimeout,
		WriteTimeout:      c.httpConfig.WriteTimeout,
		IdleTimeout:       c.httpConfig.IdleTimeout,
	}

	return c
}

// Start implements the Component interface.
func (c *MetricsServerComponent) Start() error {
	c.logger.Infof("Exposing metrics and probes on http://0.0.0.0:%d", c.port)

	go func() {
		if err := c.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			err = fmt.Errorf("failed to start metrics server: %w", err)
			fsentry.CaptureAndFlush(err, fsentry.DefaultFlushTimeout)
			c.logger.Fatal(err)
		}
	}()

	return nil
}

// Stop implements the Component interface.
func (c *MetricsServerComponent) Stop() error {
	// The probes are the last thing an orchestrator sees, so drain rather than
	// cutting live requests off mid-response.
	ctx, cancel := context.WithTimeout(context.Background(), metricsServerShutdownTimeout)
	defer cancel()

	return c.server.Shutdown(ctx)
}

// Health implements the Component interface.
func (c *MetricsServerComponent) Health() error {
	return nil
}

// HealthContext implements the HealthCheckerContext interface.
func (c *MetricsServerComponent) HealthContext(context.Context) error {
	return nil
}

// Name implements the Component interface.
func (c *MetricsServerComponent) Name() string {
	return MetricsServerComponentName
}
