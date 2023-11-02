package foundation

import (
	fredis "github.com/foundation-go/foundation/redis"
	"github.com/getsentry/sentry-go"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

func (s *Service) GetRedis() *redis.Client {
	component := s.GetComponent(fredis.ComponentName)
	if component == nil {
		err := errors.New("redis component is not registered")
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	comp, ok := component.(*fredis.Component)
	if !ok {
		err := errors.New("redis component is not of type *foundation_redis.Component")
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	return comp.Connection
}
