package foundation

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

type InsightServerComponent struct {
	healthHandler http.HandlerFunc
	logger        *logrus.Entry
	port          int
	server        *http.Server
}

type InsightServerComponentOption func(*InsightServerComponent)

// WithInsightServerLogger sets the logger for the InsightServer component.
func WithInsightServerLogger(logger *logrus.Entry) InsightServerComponentOption {
	return func(c *InsightServerComponent) {
		c.logger = logger.WithField("component", c.Name())
	}
}

// WithInsightServerPort sets the port for the InsightServer component.
func WithInsightServerPort(port int) InsightServerComponentOption {
	return func(c *InsightServerComponent) {
		c.port = port
	}
}

// WithInsightServerHealthHandler sets the health handler for the InsightServer component.
func WithInsightServerHealthHandler(handler http.HandlerFunc) InsightServerComponentOption {
	return func(c *InsightServerComponent) {
		c.healthHandler = handler
	}
}

func NewInsightServerComponent(opts ...InsightServerComponentOption) *InsightServerComponent {
	c := &InsightServerComponent{
		port: 51088,
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
func (c *InsightServerComponent) Start() error {
	c.logger.Infof("Exposing insight HTTP server on http://0.0.0.0:%d", c.port)

	go func() {
		if err := c.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			c.logger.Fatalf("Failed to start insight HTTP server: %v", err)
		}
	}()

	return nil
}

// Stop implements the Component interface.
func (c *InsightServerComponent) Stop() error {
	c.logger.Info("Stopping insight HTTP server...")

	return c.server.Close()
}

// Health implements the Component interface.
func (c *InsightServerComponent) Health() error {
	return nil
}

// Name implements the Component interface.
func (c *InsightServerComponent) Name() string {
	return "insight_server"
}
