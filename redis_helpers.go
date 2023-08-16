package foundation

import (
	"github.com/getsentry/sentry-go"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	fredis "github.com/ri-nat/foundation/redis"
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
		err := errors.New("redis component is not of type *fredis.RedisComponent")
		sentry.CaptureException(err)
		s.Logger.Fatal(err)
	}

	return comp.Connection
}
