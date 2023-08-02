package sentry

import (
	"github.com/getsentry/sentry-go"
)

type Component struct {
	DSN string
}

func NewComponent(dsn string) *Component {
	return &Component{
		DSN: dsn,
	}
}

// Start implements the Component interface.
func (c *Component) Start() error {
	return sentry.Init(sentry.ClientOptions{
		Dsn: c.DSN,
	})
}

// Stop implements the Component interface.
func (c *Component) Stop() error {
	sentry.Flush(2)

	return nil
}

// Health implements the Component interface.
func (c *Component) Health() error {
	return nil
}

// Name implements the Component interface.
func (c *Component) Name() string {
	return "sentry"
}
