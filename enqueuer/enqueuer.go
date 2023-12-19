package enqueuer

import (
	"fmt"

	"github.com/gocraft/work"
	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
)

const (
	ComponentName = "enqueuer"
)

const (
	defaultPoolSize  = 5
	defaultNamespace = "foundation_jobs_worker"
)

type Component struct {
	Enqueuer *work.Enqueuer

	url       string
	namespace string
	poolSize  int
	logger    *logrus.Entry
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

// WithPoolSize sets the pool size for the Redis component.
func WithPoolSize(poolSize int) ComponentOption {
	return func(c *Component) {
		c.poolSize = poolSize
	}
}

// WithNamespace sets the namespace for enqueuer.
func WithNamespace(namespace string) ComponentOption {
	return func(c *Component) {
		c.namespace = namespace
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
	if c.poolSize == 0 {
		c.poolSize = defaultPoolSize
	}

	if c.namespace == "" {
		c.namespace = defaultNamespace
	}

	c.Enqueuer = work.NewEnqueuer(c.namespace, &redis.Pool{
		MaxActive: c.poolSize,
		MaxIdle:   c.poolSize,
		Wait:      true,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", c.url)
		},
	})

	return c.Health()
}

// Stop implements the Component interface.
func (c *Component) Stop() error {
	c.logger.Info("Disconnecting from Redis...")

	return c.Enqueuer.Pool.Close()
}

// Health implements the Component interface.
func (c *Component) Health() error {
	if c.Enqueuer.Pool == nil {
		return fmt.Errorf("connection is not initialized")
	}

	conn := c.Enqueuer.Pool.Get()
	defer conn.Close()

	_, err := conn.Do("PING")
	if err != nil {
		return err
	}

	return nil
}

// Name implements the Component interface.
func (c *Component) Name() string {
	return ComponentName
}
