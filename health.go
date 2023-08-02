package foundation

import (
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
)

func (app *Application) healthHandler(w http.ResponseWriter, r *http.Request) {
	for _, component := range app.Components {
		started := time.Now()

		if err := component.Health(); err != nil {
			err = fmt.Errorf("health check failed for `%s`: %w", component.Name(), err)
			sentry.CaptureException(err)
			app.Logger.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		app.Logger.Debugf("Health check for `%s` took %dms", component.Name(), time.Since(started).Milliseconds())
	}

	w.WriteHeader(http.StatusOK)
}
