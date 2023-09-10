package foundation

import (
	"github.com/getsentry/sentry-go"
	ferr "github.com/ri-nat/foundation/errors"
)

func (s *Service) HandleError(err ferr.FoundationError, prefix string) {
	// Log internal errors
	if _, ok := err.(*ferr.InternalError); ok {
		if prefix != "" {
			s.Logger.Errorf("%s: %s", prefix, err.Error())
		} else {
			s.Logger.Error(err.Error())
		}

		sentry.CaptureException(err)
	}
}
