package hydra

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	hydraclient "github.com/ory/hydra-client-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// introspectionServer stands in for Hydra and counts the requests it receives.
func introspectionServer(t *testing.T, body string) (*httptest.Server, *atomic.Int64) {
	t.Helper()

	var calls atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return server, &calls
}

// Introspection sits on the authentication path of every request, and the
// client used to be rebuilt per call — no pooling, and no cache either, so
// every request cost a synchronous round-trip.
func TestIntrospectTokenCachesResults(t *testing.T) {
	server, calls := introspectionServer(t, `{"active":true,"sub":"user-1"}`)

	client := NewClient(server.URL, DefaultTimeout, time.Minute)

	for i := 0; i < 5; i++ {
		result, err := client.IntrospectToken(context.Background(), "token")
		require.NoError(t, err)
		assert.True(t, result.Active)
		assert.Equal(t, "user-1", result.GetSub())
	}

	assert.Equal(t, int64(1), calls.Load(), "the result should be reused")
}

func TestIntrospectTokenCachesPerToken(t *testing.T) {
	server, calls := introspectionServer(t, `{"active":true,"sub":"user-1"}`)

	client := NewClient(server.URL, DefaultTimeout, time.Minute)

	_, err := client.IntrospectToken(context.Background(), "token-a")
	require.NoError(t, err)
	_, err = client.IntrospectToken(context.Background(), "token-b")
	require.NoError(t, err)

	assert.Equal(t, int64(2), calls.Load())
}

func TestIntrospectTokenWithCachingDisabled(t *testing.T) {
	server, calls := introspectionServer(t, `{"active":true}`)

	client := NewClient(server.URL, DefaultTimeout, 0)

	for i := 0; i < 3; i++ {
		_, err := client.IntrospectToken(context.Background(), "token")
		require.NoError(t, err)
	}

	assert.Equal(t, int64(3), calls.Load())
}

// Caching must never outlive the token: a result kept past `exp` would let an
// expired token keep working.
func TestCacheNeverOutlivesTheToken(t *testing.T) {
	c := newCache(time.Hour)

	tokenExpiry := time.Now().Add(30 * time.Second)
	exp := tokenExpiry.Unix()
	c.put("token", introspected(true, &exp))

	c.mu.Lock()
	entry, ok := c.entries[c.key("token")]
	c.mu.Unlock()

	require.True(t, ok)
	assert.WithinDuration(t, tokenExpiry, entry.expiresAt, time.Second,
		"the entry must expire with the token, not an hour later")
}

// Without an `exp` the configured TTL applies.
func TestCacheUsesTheConfiguredTTLWithoutAnExpiry(t *testing.T) {
	c := newCache(time.Minute)

	c.put("token", introspected(true, nil))

	c.mu.Lock()
	entry := c.entries[c.key("token")]
	c.mu.Unlock()

	assert.WithinDuration(t, time.Now().Add(time.Minute), entry.expiresAt, time.Second)
}

func TestCacheEntriesExpire(t *testing.T) {
	c := newCache(20 * time.Millisecond)

	c.put("token", introspected(true, nil))

	_, ok := c.get("token")
	assert.True(t, ok)

	time.Sleep(40 * time.Millisecond)

	_, ok = c.get("token")
	assert.False(t, ok)
}

func TestCacheRejectsAlreadyExpiredTokens(t *testing.T) {
	c := newCache(time.Hour)

	past := time.Now().Add(-time.Hour).Unix()
	c.put("token", introspected(true, &past))

	_, ok := c.get("token")
	assert.False(t, ok)
}

// A flood of distinct tokens must not grow the cache without limit.
func TestCacheIsBounded(t *testing.T) {
	c := newCache(time.Hour)

	for i := 0; i < maxCacheEntries+100; i++ {
		c.put(string(rune(i))+"-token", introspected(true, nil))
	}

	c.mu.Lock()
	size := len(c.entries)
	c.mu.Unlock()

	assert.LessOrEqual(t, size, maxCacheEntries)
}

// The raw token is not used as the map key.
func TestCacheKeyIsHashed(t *testing.T) {
	c := newCache(time.Minute)

	assert.NotEqual(t, "secret-token", c.key("secret-token"))
	assert.Len(t, c.key("secret-token"), 64)
	assert.Equal(t, c.key("secret-token"), c.key("secret-token"))
}

func TestIntrospectTokenWithoutAToken(t *testing.T) {
	server, calls := introspectionServer(t, `{"active":true}`)

	client := NewClient(server.URL, DefaultTimeout, time.Minute)

	result, err := client.IntrospectToken(context.Background(), "")
	require.NoError(t, err)

	assert.False(t, result.Active)
	assert.Zero(t, calls.Load(), "an empty token needs no round-trip")
}

func TestEnvDuration(t *testing.T) {
	t.Setenv("X", "")
	assert.Equal(t, time.Second, envDuration("X", time.Second))

	t.Setenv("X", "250ms")
	assert.Equal(t, 250*time.Millisecond, envDuration("X", time.Second))

	// A bare number is read as seconds.
	t.Setenv("X", "5")
	assert.Equal(t, 5*time.Second, envDuration("X", time.Second))

	t.Setenv("X", "nonsense")
	assert.Equal(t, time.Second, envDuration("X", time.Second))
}

// introspected builds an introspection result for the cache tests.
func introspected(active bool, exp *int64) *hydraclient.IntrospectedOAuth2Token {
	result := &hydraclient.IntrospectedOAuth2Token{Active: active}
	if exp != nil {
		result.Exp = exp
	}

	return result
}
