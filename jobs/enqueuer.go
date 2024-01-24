package jobs

import (
	"fmt"

	"github.com/gocraft/work"
	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
)

const (
	ComponentName = "jobs-enqueuer"
)

const (
	DefaultPoolSize  = 5
	DefaultNamespace = "__foundation_jobs__"
)

type Component struct {
	Enqueuer *work.Enqueuer

	address   string
	namespace string
	poolSize  int
	logger    *logrus.Entry
}

// ComponentOption is an option to `Component`.
type ComponentOption func(*Component)

// WithLogger sets the logger for the JobsEnqueuer component
func WithLogger(logger *logrus.Entry) ComponentOption {
	return func(c *Component) {
		c.logger = logger.WithField("component", c.Name())
	}
}

// WithAddress sets the redis address for the JobsEnqueuer component.
func WithAddress(address string) ComponentOption {
	return func(c *Component) {
		c.address = address
	}
}

// WithPoolSize sets the pool size for the JobsEnqueuer component.
func WithPoolSize(poolSize int) ComponentOption {
	return func(c *Component) {
		c.poolSize = poolSize
	}
}

// WithNamespace sets the namespace for JobsEnqueuer component.
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
		c.poolSize = DefaultPoolSize
	}

	if c.namespace == "" {
		c.namespace = DefaultNamespace
	}

	c.Enqueuer = work.NewEnqueuer(c.namespace, &redis.Pool{
		MaxActive: c.poolSize,
		MaxIdle:   c.poolSize,
		Wait:      true,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", c.address)
		},
	})

	return c.Health()
}

// Stop implements the Component interface.
func (c *Component) Stop() error {
	c.logger.Info("Disconnecting jobs enqueuer from redis...")

	return c.Enqueuer.Pool.Close()
}

// Health implements the Component interface.
func (c *Component) Health() error {
	if c.Enqueuer.Pool == nil {
		return fmt.Errorf("jobs enqueuer redis connection is not initialized")
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
