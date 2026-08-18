package foundation

import (
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	fredis "github.com/foundation-go/foundation/redis"
)

// GetRedis returns the Redis client.
//
// It panics when the client is unavailable; see GetPostgreSQL for why, and use
// TryGetRedis where the absence has to be handled.
func (s *Service) GetRedis() *redis.Client {
	client, err := s.TryGetRedis()
	if err != nil {
		panic(err)
	}

	return client
}

// TryGetRedis returns the Redis client, or an error explaining why it is not
// available.
func (s *Service) TryGetRedis() (*redis.Client, error) {
	component := s.GetComponent(fredis.ComponentName)
	if component == nil {
		return nil, errors.New("no Redis component is registered: set REDIS_URL or use foundation.WithRedis()")
	}

	comp, ok := component.(*fredis.Component)
	if !ok {
		return nil, fmt.Errorf("component `%s` is a %T, not a *redis.Component", fredis.ComponentName, component)
	}

	if comp.Connection == nil {
		return nil, errors.New("the Redis component has not been started yet")
	}

	return comp.Connection, nil
}
