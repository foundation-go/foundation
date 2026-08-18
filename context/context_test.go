package context

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An empty context used to be enough to bring the whole process down: every
// getter was an unchecked type assertion, and `addDefaultHeaders` calls
// GetCorrelationID for every published event.
func TestGettersDoNotPanicOnEmptyContext(t *testing.T) {
	ctx := context.Background()

	assert.NotPanics(t, func() {
		assert.Equal(t, "", GetCorrelationID(ctx))
		assert.Equal(t, "", GetAccessToken(ctx))
		assert.Equal(t, "", GetRequestID(ctx))
		assert.Equal(t, uuid.Nil, GetClientID(ctx))
		assert.Equal(t, uuid.Nil, GetUserID(ctx))
		assert.False(t, GetAuthenticated(ctx))
		assert.Nil(t, GetScopes(ctx))
		assert.Nil(t, GetTX(ctx))
		assert.NotNil(t, GetLogger(ctx))
	})
}

func TestGettersDoNotPanicOnNilContext(t *testing.T) {
	assert.NotPanics(t, func() {
		//nolint:staticcheck // deliberately passing a nil context
		assert.Equal(t, "", GetCorrelationID(nil))
	})
}

func TestGettersReturnStoredValues(t *testing.T) {
	userID := uuid.New()
	clientID := uuid.New()
	logger := logrus.NewEntry(logrus.New())

	ctx := context.Background()
	ctx = WithCorrelationID(ctx, "corr-1")
	ctx = WithRequestID(ctx, "req-1")
	ctx = WithAccessToken(ctx, "token")
	ctx = WithUserID(ctx, userID)
	ctx = WithClientID(ctx, clientID)
	ctx = WithAuthenticated(ctx, true)
	ctx = WithScopes(ctx, Oauth2Scopes{"read", "write"})
	ctx = WithLogger(ctx, logger)

	assert.Equal(t, "corr-1", GetCorrelationID(ctx))
	assert.Equal(t, "req-1", GetRequestID(ctx))
	assert.Equal(t, "token", GetAccessToken(ctx))
	assert.Equal(t, userID, GetUserID(ctx))
	assert.Equal(t, clientID, GetClientID(ctx))
	assert.True(t, GetAuthenticated(ctx))
	assert.Equal(t, Oauth2Scopes{"read", "write"}, GetScopes(ctx))
	assert.Same(t, logger, GetLogger(ctx))
}

func TestLookupReportsPresence(t *testing.T) {
	_, ok := LookupCorrelationID(context.Background())
	assert.False(t, ok)

	value, ok := LookupCorrelationID(WithCorrelationID(context.Background(), "corr-1"))
	require.True(t, ok)
	assert.Equal(t, "corr-1", value)
}

// An empty correlation ID is a legitimate value and must be distinguishable
// from an absent one.
func TestLookupDistinguishesEmptyFromAbsent(t *testing.T) {
	value, ok := LookupCorrelationID(WithCorrelationID(context.Background(), ""))
	assert.True(t, ok)
	assert.Equal(t, "", value)
}

func TestMustGetPanicsWithDescriptiveMessage(t *testing.T) {
	assert.PanicsWithValue(
		t,
		"foundation/context: `correlation_id` is not set in the context",
		func() { MustGetCorrelationID(context.Background()) },
	)
}

func TestGetLoggerFallsBackWhenAbsent(t *testing.T) {
	assert.Same(t, FallbackLogger(), GetLogger(context.Background()))

	_, ok := LookupLogger(context.Background())
	assert.False(t, ok)
}

// A nil *logrus.Entry stored in the context must not be handed back to callers.
func TestGetLoggerFallsBackOnNilEntry(t *testing.T) {
	ctx := WithLogger(context.Background(), nil)

	assert.Same(t, FallbackLogger(), GetLogger(ctx))

	_, ok := LookupLogger(ctx)
	assert.False(t, ok)
}

func TestOauth2Scopes(t *testing.T) {
	scopes := Oauth2Scopes{"read", "write"}

	assert.True(t, scopes.ContainsAll("read", "write"))
	assert.False(t, scopes.ContainsAll("read", "delete"))
	assert.True(t, scopes.ContainsAny("delete", "write"))
	assert.False(t, scopes.ContainsAny("delete"))

	// ContainsAll over an empty requirement list is vacuously true; ContainsAny
	// over an empty list is false.
	assert.True(t, scopes.ContainsAll())
	assert.False(t, scopes.ContainsAny())
}
