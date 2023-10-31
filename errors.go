package foundation

import (
	"errors"
	"github.com/getsentry/sentry-go"
	ferr "github.com/ri-nat/foundation/errors"
)

func (s *Service) HandleError(err ferr.FoundationError, prefix string) {
	// Log internal errors
	var internalError *ferr.InternalError
	if errors.As(err, &internalError) {
		if prefix != "" {
			s.Logger.Errorf("%s: %s", prefix, err.Error())
		} else {
			s.Logger.Error(err.Error())
		}

		sentry.CaptureException(err)
	}
}
