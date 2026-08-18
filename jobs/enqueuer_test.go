package jobs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Health checked `c.Enqueuer.Pool == nil`, which dereferences a nil Enqueuer
// before Start has run. The metrics server — and therefore the probe endpoint —
// is registered before this component, so probes really do arrive in that
// window and used to panic inside the HTTP handler.
func TestHealthBeforeStart(t *testing.T) {
	c := NewComponent()

	var err error

	require.NotPanics(t, func() { err = c.Health() })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestHealthContextBeforeStart(t *testing.T) {
	c := NewComponent()

	require.NotPanics(t, func() {
		assert.Error(t, c.HealthContext(context.Background()))
	})
}

func TestStopBeforeStart(t *testing.T) {
	c := NewComponent()

	require.NotPanics(t, func() { assert.NoError(t, c.Stop()) })
}

func TestStartRequiresARedisPool(t *testing.T) {
	err := NewComponent().Start()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis pool")
}

func TestNameAndNamespaceDefaults(t *testing.T) {
	c := NewComponent()

	assert.Equal(t, ComponentName, c.Name())
	assert.Equal(t, "", c.namespace, "the default is applied at Start, not at construction")

	c = NewComponent(WithNamespace("custom"))
	assert.Equal(t, "custom", c.namespace)
}
