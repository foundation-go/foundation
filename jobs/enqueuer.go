package jobs

import (
	"context"
	"errors"
	"time"

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

	// DefaultHealthTimeout bounds a health check made without a context.
	DefaultHealthTimeout = 2 * time.Second
)

type Component struct {
	Enqueuer *work.Enqueuer

	redisPool *redis.Pool
	namespace string
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

// WithRedisPool sets the redis pool for JobsEnqueuer component.
func WithRedisPool(redisPool *redis.Pool) ComponentOption {
	return func(c *Component) {
		c.redisPool = redisPool
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
	if c.namespace == "" {
		c.namespace = DefaultNamespace
	}

	if c.redisPool == nil {
		return errors.New("jobs enqueuer requires a redis pool")
	}

	c.Enqueuer = work.NewEnqueuer(c.namespace, c.redisPool)

	return c.Health()
}

// Stop implements the Component interface.
func (c *Component) Stop() error {
	if c.Enqueuer == nil || c.Enqueuer.Pool == nil {
		return nil
	}

	c.log().Info("Disconnecting jobs enqueuer from redis...")

	return c.Enqueuer.Pool.Close()
}

// Health implements the Component interface.
func (c *Component) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultHealthTimeout)
	defer cancel()

	return c.HealthContext(ctx)
}

// HealthContext implements the HealthCheckerContext interface.
func (c *Component) HealthContext(ctx context.Context) error {
	// N.B.: the nil check used to be on c.Enqueuer.Pool, which dereferences a
	// nil c.Enqueuer before Start has run. The metrics server — and therefore
	// the probe endpoint — is registered before this component, so that window
	// is reachable in practice.
	if c.Enqueuer == nil || c.Enqueuer.Pool == nil {
		return errors.New("jobs enqueuer redis connection is not initialized")
	}

	conn, err := c.Enqueuer.Pool.GetContext(ctx)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // returning the connection to the pool

	_, err = redis.DoContext(conn, ctx, "PING")

	return err
}

// Name implements the Component interface.
func (c *Component) Name() string {
	return ComponentName
}
