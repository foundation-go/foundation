package foundation

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (app *Application) StartMetricsServer() {
	if !GetEnvOrBool("METRICS_ENABLED", false) {
		return
	}

	metricsPort := GetEnvOrInt("METRICS_PORT", 51077)
	app.Logger.Infof("Exposing Prometheus metrics on http://0.0.0.0:%d", metricsPort)

	http.Handle("/", promhttp.Handler())

	err := http.ListenAndServe(fmt.Sprintf(":%d", metricsPort), nil)
	if err != nil {
		app.Logger.Fatalf("Failed to start metrics server: %v", err)
	}
}
