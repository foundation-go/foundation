package foundation

import (
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

const (
	MetricsServerComponentName = "metrics-server"
)

type MetricsServerComponent struct {
	healthHandler http.HandlerFunc
	logger        *logrus.Entry
	port          int
	server        *http.Server
}

type MetricsServerComponentOption func(*MetricsServerComponent)

// WithMetricsServerLogger sets the logger for the MetricsServer component.
func WithMetricsServerLogger(logger *logrus.Entry) MetricsServerComponentOption {
	return func(c *MetricsServerComponent) {
		c.logger = logger.WithField("component", c.Name())
	}
}

// WithMetricsServerPort sets the port for the MetricsServer component.
func WithMetricsServerPort(port int) MetricsServerComponentOption {
	return func(c *MetricsServerComponent) {
		c.port = port
	}
}

// WithMetricsServerHealthHandler sets the health handler for the MetricsServer component.
func WithMetricsServerHealthHandler(handler http.HandlerFunc) MetricsServerComponentOption {
	return func(c *MetricsServerComponent) {
		c.healthHandler = handler
	}
}

func NewMetricsServerComponent(opts ...MetricsServerComponentOption) *MetricsServerComponent {
	c := &MetricsServerComponent{
		port: 51077,
	}

	for _, opt := range opts {
		opt(c)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", c.healthHandler)
	mux.Handle("/metrics", promhttp.Handler())

	c.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", c.port),
		Handler: mux,
	}

	return c
}

// Start implements the Component interface.
func (c *MetricsServerComponent) Start() error {
	c.logger.Infof("Exposing metrics HTTP server on http://0.0.0.0:%d", c.port)

	go func() {
		if err := c.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			err = fmt.Errorf("failed to start metrics server: %w", err)
			sentry.CaptureException(err)
			c.logger.Fatal(err)
		}
	}()

	return nil
}

// Stop implements the Component interface.
func (c *MetricsServerComponent) Stop() error {
	return c.server.Close()
}

// Health implements the Component interface.
func (c *MetricsServerComponent) Health() error {
	return nil
}

// Name implements the Component interface.
func (c *MetricsServerComponent) Name() string {
	return MetricsServerComponentName
}
