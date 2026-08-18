package foundation

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeComponent is a Component whose behaviour each test dictates.
type fakeComponent struct {
	name       string
	startErr   error
	stopErr    error
	stopPanics bool
	stopBlocks chan struct{}

	started bool
	stopped bool
}

func (c *fakeComponent) Name() string { return c.name }

func (c *fakeComponent) Start() error {
	if c.startErr != nil {
		return c.startErr
	}

	c.started = true

	return nil
}

func (c *fakeComponent) Stop() error {
	if c.stopPanics {
		panic("component blew up on stop")
	}

	if c.stopBlocks != nil {
		<-c.stopBlocks
	}

	c.stopped = true

	return c.stopErr
}

func (c *fakeComponent) Health() error { return nil }

func newLifecycleTestService(components ...Component) *Service {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	svc := &Service{
		Name:       "test",
		Config:     NewConfig(),
		Logger:     logrus.NewEntry(logger),
		Components: components,
	}
	svc.Config.Metrics.Enabled = false
	svc.Config.ShutdownTimeout = time.Second

	return svc
}

// A service that fails to start halfway through used to leave every component
// before the failure running: database pools, Kafka readers, listening sockets.
func TestStartComponentsUnwindsOnFailure(t *testing.T) {
	first := &fakeComponent{name: "first"}
	second := &fakeComponent{name: "second"}
	failing := &fakeComponent{name: "failing", startErr: errors.New("nope")}
	never := &fakeComponent{name: "never"}

	svc := newLifecycleTestService(first, second, failing, never)

	err := svc.StartComponents()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failing")

	assert.True(t, first.stopped, "components started before the failure must be stopped")
	assert.True(t, second.stopped)
	assert.False(t, never.started, "components after the failure must not be started")
	assert.False(t, never.stopped)
}

func TestStartComponentsStartsEverythingOnSuccess(t *testing.T) {
	first := &fakeComponent{name: "first"}
	second := &fakeComponent{name: "second"}

	svc := newLifecycleTestService(first, second)

	require.NoError(t, svc.StartComponents())

	assert.True(t, first.started)
	assert.True(t, second.started)
	assert.False(t, first.stopped)
	assert.False(t, second.stopped)
}

// Components are stopped in reverse order so that dependents go down before
// their dependencies.
func TestStopComponentsRunsInReverseOrder(t *testing.T) {
	var order []string

	record := func(name string) *fakeComponent {
		return &fakeComponent{name: name}
	}

	first, second := record("first"), record("second")
	svc := newLifecycleTestService(first, second)

	// Track the order through the logger-free path by observing stop calls.
	svc.Components = []Component{
		&orderedComponent{fakeComponent: first, order: &order},
		&orderedComponent{fakeComponent: second, order: &order},
	}

	svc.StopComponents()

	assert.Equal(t, []string{"second", "first"}, order)
}

type orderedComponent struct {
	*fakeComponent
	order *[]string
}

func (c *orderedComponent) Stop() error {
	*c.order = append(*c.order, c.name)

	return c.fakeComponent.Stop()
}

// One component blowing up must not prevent the others from shutting down.
func TestStopComponentsContainsPanics(t *testing.T) {
	healthy := &fakeComponent{name: "healthy"}
	exploding := &fakeComponent{name: "exploding", stopPanics: true}

	svc := newLifecycleTestService(healthy, exploding)

	assert.NotPanics(t, func() { svc.StopComponents() })
	assert.True(t, healthy.stopped, "the remaining components must still be stopped")
}

func TestStopComponentsContinuesAfterAnError(t *testing.T) {
	failing := &fakeComponent{name: "failing", stopErr: errors.New("nope")}
	healthy := &fakeComponent{name: "healthy"}

	svc := newLifecycleTestService(healthy, failing)

	svc.StopComponents()

	assert.True(t, healthy.stopped)
}

// A component that refuses to stop must not hold the process open until the
// supervisor kills it — nothing after StopComponents would get to run.
func TestStopComponentsIsBounded(t *testing.T) {
	blocked := make(chan struct{})
	t.Cleanup(func() { close(blocked) })

	svc := newLifecycleTestService(&fakeComponent{name: "stuck", stopBlocks: blocked})
	svc.Config.ShutdownTimeout = 30 * time.Millisecond

	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.StopComponents()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StopComponents blocked on a component that never returns")
	}
}

func TestGetComponent(t *testing.T) {
	wanted := &fakeComponent{name: "wanted"}
	svc := newLifecycleTestService(&fakeComponent{name: "other"}, wanted)

	assert.Same(t, Component(wanted), svc.GetComponent("wanted"))
	assert.Nil(t, svc.GetComponent("missing"))
}
