package redis

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

const (
	ComponentName = "redis"

	// DefaultHealthTimeout bounds a health check made without a context.
	DefaultHealthTimeout = 2 * time.Second
)

type Component struct {
	Connection *redis.Client

	url    string
	logger *logrus.Entry
}

// ComponentOption is an option to `Component`.
type ComponentOption func(*Component)

// WithLogger sets the logger for the Redis component.
func WithLogger(logger *logrus.Entry) ComponentOption {
	return func(c *Component) {
		c.logger = logger.WithField("component", c.Name())
	}
}

// WithURL sets the database URL for the Redis component.
func WithURL(url string) ComponentOption {
	return func(c *Component) {
		c.url = url
	}
}

func NewComponent(opts ...ComponentOption) *Component {
	c := &Component{}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// log returns the component logger, or a usable default when the component was
// built without one.
func (c *Component) log() *logrus.Entry {
	if c.logger == nil {
		return logrus.NewEntry(logrus.StandardLogger()).WithField("component", c.Name())
	}

	return c.logger
}

// Start implements the Component interface.
func (c *Component) Start() error {
	opts, err := redis.ParseURL(c.url)
	if err != nil {
		return err
	}

	c.Connection = redis.NewClient(opts)

	return c.Health()
}

// Stop implements the Component interface.
func (c *Component) Stop() error {
	if c.Connection == nil {
		return nil
	}

	c.log().Info("Disconnecting from Redis...")

	return c.Connection.Close()
}

// Health implements the Component interface.
func (c *Component) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultHealthTimeout)
	defer cancel()

	return c.HealthContext(ctx)
}

// HealthContext implements the HealthCheckerContext interface.
func (c *Component) HealthContext(ctx context.Context) error {
	if c.Connection == nil {
		return errors.New("connection is not initialized")
	}

	return c.Connection.Ping(ctx).Err()
}

// Name implements the Component interface.
func (c *Component) Name() string {
	return ComponentName
}
