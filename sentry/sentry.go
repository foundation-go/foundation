package sentry

import (
	"time"

	"github.com/getsentry/sentry-go"
)

const (
	// ComponentName is the name the Sentry component is registered under.
	ComponentName = "sentry"

	// DefaultFlushTimeout is how long the component waits for buffered events
	// to reach Sentry before giving up.
	DefaultFlushTimeout = 2 * time.Second
)

// Seams over the Sentry SDK, so that tests can observe what the package asks
// the SDK to do.
var (
	captureException = sentry.CaptureException
	flush            = sentry.Flush
)

type Component struct {
	DSN string

	environment      string
	release          string
	flushTimeout     time.Duration
	attachStacktrace bool
}

// ComponentOption is an option to `Component`.
type ComponentOption func(*Component)

// WithEnvironment sets the environment reported to Sentry, so that events from
// development, test and production do not end up in the same bucket.
func WithEnvironment(environment string) ComponentOption {
	return func(c *Component) {
		c.environment = environment
	}
}

// WithRelease sets the release reported to Sentry.
func WithRelease(release string) ComponentOption {
	return func(c *Component) {
		c.release = release
	}
}

// WithFlushTimeout sets how long Stop waits for buffered events to be
// delivered.
func WithFlushTimeout(timeout time.Duration) ComponentOption {
	return func(c *Component) {
		c.flushTimeout = timeout
	}
}

// WithAttachStacktrace controls whether stack traces are attached to messages
// that do not carry one.
func WithAttachStacktrace(attach bool) ComponentOption {
	return func(c *Component) {
		c.attachStacktrace = attach
	}
}

func NewComponent(dsn string, opts ...ComponentOption) *Component {
	c := &Component{
		DSN:              dsn,
		flushTimeout:     DefaultFlushTimeout,
		attachStacktrace: true,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// FlushTimeout returns the configured flush timeout.
func (c *Component) FlushTimeout() time.Duration {
	if c.flushTimeout <= 0 {
		return DefaultFlushTimeout
	}

	return c.flushTimeout
}

// Start implements the Component interface.
func (c *Component) Start() error {
	return sentry.Init(sentry.ClientOptions{
		Dsn:              c.DSN,
		Environment:      c.environment,
		Release:          c.release,
		AttachStacktrace: c.attachStacktrace,
	})
}

// Stop implements the Component interface.
func (c *Component) Stop() error {
	// N.B.: sentry.Flush takes a time.Duration. Passing a bare `2` here meant
	// two *nanoseconds*, so buffered events were dropped on every shutdown.
	flush(c.FlushTimeout())

	return nil
}

// Health implements the Component interface.
func (c *Component) Health() error {
	return nil
}

// Name implements the Component interface.
func (c *Component) Name() string {
	return ComponentName
}

// CaptureAndFlush reports err to Sentry and blocks until the report has been
// delivered or timeout elapses.
//
// The Sentry transport is asynchronous, so any code path that reports an error
// and then terminates the process must flush explicitly — otherwise exactly the
// errors worth knowing about are the ones that never arrive.
func CaptureAndFlush(err error, timeout time.Duration) {
	if err == nil {
		return
	}

	if timeout <= 0 {
		timeout = DefaultFlushTimeout
	}

	captureException(err)
	flush(timeout)
}
