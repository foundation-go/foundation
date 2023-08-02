package foundation

import (
	"net/http"
	"time"
)

func (app *Application) healthHandler(w http.ResponseWriter, r *http.Request) {
	for _, component := range app.Components {
		started := time.Now()

		if err := component.Health(); err != nil {
			app.Logger.Errorf("health check failed for `%s`: %s", component.Name(), err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		app.Logger.Debugf("health check for `%s` took %dms", component.Name(), time.Since(started).Milliseconds())
	}

	w.WriteHeader(http.StatusOK)
}
