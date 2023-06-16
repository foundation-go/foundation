package foundation

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func StartMetricsServer() {
	metricsPort := GetEnvOrInt("METRICS_PORT", 51077)
	log.Infof("Exposing Prometheus metrics on http://0.0.0.0:%d", metricsPort)

	http.Handle("/", promhttp.Handler())

	err := http.ListenAndServe(fmt.Sprintf(":%d", metricsPort), nil)
	if err != nil {
		log.Fatalf("Failed to start metrics server: %v", err)
	}
}
