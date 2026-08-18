package foundation

import (
	"errors"
	"fmt"

	"github.com/gocraft/work"

	fjobs "github.com/foundation-go/foundation/jobs"
)

// GetJobsEnqueuer returns the jobs enqueuer.
//
// It panics when the enqueuer is unavailable; see GetPostgreSQL for why, and
// use TryGetJobsEnqueuer where the absence has to be handled.
func (s *Service) GetJobsEnqueuer() *work.Enqueuer {
	enqueuer, err := s.TryGetJobsEnqueuer()
	if err != nil {
		panic(err)
	}

	return enqueuer
}

// TryGetJobsEnqueuer returns the jobs enqueuer, or an error explaining why it
// is not available.
func (s *Service) TryGetJobsEnqueuer() (*work.Enqueuer, error) {
	component := s.GetComponent(fjobs.ComponentName)
	if component == nil {
		return nil, errors.New("jobs enqueuer component is not registered: use foundation.WithJobsEnqueuer()")
	}

	comp, ok := component.(*fjobs.Component)
	if !ok {
		return nil, fmt.Errorf("component `%s` is a %T, not a *jobs.Component", fjobs.ComponentName, component)
	}

	if comp.Enqueuer == nil {
		return nil, errors.New("jobs enqueuer component has not been started yet")
	}

	return comp.Enqueuer, nil
}
