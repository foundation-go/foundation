package foundation

import (
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
)

func (s *Service) healthHandler(w http.ResponseWriter, r *http.Request) {
	for _, component := range s.Components {
		started := time.Now()

		if err := component.Health(); err != nil {
			err = fmt.Errorf("health check failed for `%s`: %w", component.Name(), err)
			sentry.CaptureException(err)
			s.Logger.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		s.Logger.Debugf("Health check for `%s` took %dms", component.Name(), time.Since(started).Milliseconds())
	}

	w.WriteHeader(http.StatusOK)
}
