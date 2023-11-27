package postgresql

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

const (
	ComponentName = "redis"
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

// Start implements the Component interface.
func (c *Component) Start() error {
	opts, err := redis.ParseURL(c.url)
	if err != nil {
		return err
	}

	c.Connection = redis.NewClient(opts)

	_, err = c.Connection.Ping(context.Background()).Result()
	if err != nil {
		return err
	}

	return nil
}

// Stop implements the Component interface.
func (c *Component) Stop() error {
	c.logger.Info("Disconnecting from Redis...")

	return c.Connection.Close()
}

// Health implements the Component interface.
func (c *Component) Health() error {
	if c.Connection == nil {
		return fmt.Errorf("connection is not initialized")
	}

	status := c.Connection.Ping(context.Background())
	if status.Err() != nil {
		return status.Err()
	}

	return nil
}

// Name implements the Component interface.
func (c *Component) Name() string {
	return ComponentName
}
