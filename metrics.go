package foundation

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (app *Application) StartMetricsServer() {
	app.Logger.Infof("Exposing Prometheus metrics on http://0.0.0.0:%d", app.Config.MetricsPort)

	app.MetricsServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", app.Config.MetricsPort),
		Handler: promhttp.Handler(),
	}

	go func() {
		if err := app.MetricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.Logger.Fatalf("Failed to start metrics server: %v", err)
		}
	}()
}
