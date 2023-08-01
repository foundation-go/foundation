package foundation

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (app *Application) StartInsightServer() {
	app.Logger.Infof("Exposing insight HTTP server on http://0.0.0.0:%d", app.Config.InsightPort)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", app.healthHandler)
	mux.Handle("/metrics", promhttp.Handler())

	app.InsightServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", app.Config.InsightPort),
		Handler: mux,
	}

	go func() {
		if err := app.InsightServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.Logger.Fatalf("Failed to start insight HTTP server: %v", err)
		}
	}()
}
