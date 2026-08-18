package foundation

import (
	"context"
	"fmt"
)

// Component describes an interface for all components in the Foundation framework.
// This could be an external service, a database, a cache, etc.
type Component interface {
	// Health returns the health of the component
	Health() error
	// Name returns the name of the component
	Name() string
	// Start runs the component
	Start() error
	// Stop stops the component
	Stop() error
}

// HealthCheckerContext is an optional interface a Component may implement to
// support cancellable health checks.
//
// Health checks talk to external systems, and an unreachable system typically
// hangs rather than failing fast. Without a context there is nothing to
// interrupt, so a single unhealthy dependency can pile up probe goroutines
// indefinitely. Foundation's own components implement this interface;
// components that only implement Component are still checked, but the call
// cannot be cancelled — only abandoned.
type HealthCheckerContext interface {
	HealthContext(ctx context.Context) error
}

// checkComponentHealth runs a component's health check under ctx, containing
// both timeouts and panics.
func checkComponentHealth(ctx context.Context, component Component) error {
	if checker, ok := component.(HealthCheckerContext); ok {
		return recoverHealthCheck(func() error { return checker.HealthContext(ctx) })
	}

	// A plain Health() cannot be interrupted, so run it on its own goroutine
	// and stop waiting once the context expires. The buffered channel lets the
	// abandoned goroutine finish and exit rather than leaking on the send.
	result := make(chan error, 1)

	go func() {
		result <- recoverHealthCheck(component.Health)
	}()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return fmt.Errorf("health check did not finish in time: %w", ctx.Err())
	}
}

// recoverHealthCheck turns a panicking health check into an error. A component
// that has not been started yet is a common cause, and a panic here would
// otherwise take down the probe handler.
func recoverHealthCheck(check func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in health check: %v", r)
		}
	}()

	return check()
}

// GetComponent returns the component with the given name.
func (s *Service) GetComponent(name string) Component {
	for _, component := range s.Components {
		if component.Name() == name {
			return component
		}
	}

	return nil
}
