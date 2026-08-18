package redis

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Stop dereferenced a Connection that only exists after a successful Start, and
// logged through a logger that is nil unless WithLogger was passed.
func TestComponentIsSafeBeforeStart(t *testing.T) {
	c := NewComponent()

	require.NotPanics(t, func() { assert.NoError(t, c.Stop()) })
	require.NotPanics(t, func() { assert.ErrorContains(t, c.Health(), "not initialized") })
	require.NotPanics(t, func() {
		assert.ErrorContains(t, c.HealthContext(context.Background()), "not initialized")
	})
}

func TestStartRejectsAMalformedURL(t *testing.T) {
	assert.Error(t, NewComponent(WithURL("not-a-redis-url")).Start())
}

func TestName(t *testing.T) {
	assert.Equal(t, ComponentName, NewComponent().Name())
}
