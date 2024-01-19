package foundation

import (
	"errors"

	fjobs "github.com/foundation-go/foundation/jobs"
	"github.com/getsentry/sentry-go"
	"github.com/gocraft/work"
)

func (s *Service) GetJobsEnqueuer() *work.Enqueuer {
	component := s.GetComponent(fjobs.ComponentName)
	if component == nil {
		err := errors.New("jobs enqueuer component is not registered")
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	comp, ok := component.(*fjobs.Component)
	if !ok {
		err := errors.New("jobs enqueuer component is not of type *foundation_jobs.Component")
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	return comp.Enqueuer
}
