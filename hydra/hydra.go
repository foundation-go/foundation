package hydra

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	hydra "github.com/ory/hydra-client-go/v2"
)

const (
	// DefaultTimeout bounds a single introspection call.
	DefaultTimeout = 5 * time.Second

	// DefaultCacheTTL is how long an introspection result is reused.
	//
	// Introspection sits on the authentication path of every request, so
	// without a cache each request costs a synchronous round-trip to Hydra. The
	// trade-off is that a revoked token keeps working for up to this long; set
	// HYDRA_INTROSPECTION_CACHE_TTL to 0 to introspect every time.
	DefaultCacheTTL = 30 * time.Second

	// maxCacheEntries bounds the cache so that a flood of distinct tokens
	// cannot grow it without limit.
	maxCacheEntries = 10000
)

// Client talks to the ORY Hydra admin API.
//
// It exists because the previous implementation built a fresh
// `hydra.NewAPIClient` inside the introspection function — that is, once per
// incoming request. Nothing was pooled, so every authenticated request opened
// and closed its own TCP connection and left it in TIME_WAIT.
type Client struct {
	api   *hydra.APIClient
	cache *cache
}

// NewClient returns a Client for the given admin URL.
func NewClient(adminURL string, timeout, cacheTTL time.Duration) *Client {
	config := hydra.NewConfiguration()
	config.Servers = hydra.ServerConfigurations{{URL: adminURL}}
	config.HTTPClient = &http.Client{Timeout: timeout}

	return &Client{
		api:   hydra.NewAPIClient(config),
		cache: newCache(cacheTTL),
	}
}

// IntrospectToken returns the introspection result for a token.
func (c *Client) IntrospectToken(ctx context.Context, token string) (*hydra.IntrospectedOAuth2Token, error) {
	if token == "" {
		return &hydra.IntrospectedOAuth2Token{Active: false}, nil
	}

	if cached, ok := c.cache.get(token); ok {
		return cached, nil
	}

	req := c.api.OAuth2API.IntrospectOAuth2Token(ctx).Token(token)

	resp, httpResp, err := c.api.OAuth2API.IntrospectOAuth2TokenExecute(req)
	if httpResp != nil && httpResp.Body != nil {
		defer httpResp.Body.Close() //nolint:errcheck // the SDK has already read it
	}

	if err != nil {
		return nil, err
	}

	c.cache.put(token, resp)

	return resp, nil
}

var (
	defaultClientOnce sync.Once
	defaultClient     *Client
	defaultClientErr  error
)

// IntrospectedOAuth2Token authenticates a token against the ORY Hydra admin API
// configured by HYDRA_ADMIN_URL.
//
// The underlying client is built once and reused.
func IntrospectedOAuth2Token(ctx context.Context, token string) (*hydra.IntrospectedOAuth2Token, error) {
	defaultClientOnce.Do(func() {
		adminURL := os.Getenv("HYDRA_ADMIN_URL")
		if adminURL == "" {
			defaultClientErr = errors.New("HYDRA_ADMIN_URL is not set")

			return
		}

		defaultClient = NewClient(adminURL, envDuration("HYDRA_TIMEOUT", DefaultTimeout), envDuration("HYDRA_INTROSPECTION_CACHE_TTL", DefaultCacheTTL))
	})

	if defaultClientErr != nil {
		return nil, defaultClientErr
	}

	return defaultClient.IntrospectToken(ctx, token)
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	if value, err := time.ParseDuration(raw); err == nil {
		return value
	}

	if seconds, err := strconv.Atoi(raw); err == nil {
		return time.Duration(seconds) * time.Second
	}

	return fallback
}

// cache holds introspection results for a short while.
type cache struct {
	ttl time.Duration

	mu      sync.Mutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	result    *hydra.IntrospectedOAuth2Token
	expiresAt time.Time
}

func newCache(ttl time.Duration) *cache {
	return &cache{ttl: ttl, entries: make(map[string]cacheEntry)}
}

// key hashes the token rather than storing it. The token is in memory either
// way, but a hash keeps it out of map dumps and heap profiles.
func (c *cache) key(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}

func (c *cache) get(token string) (*hydra.IntrospectedOAuth2Token, bool) {
	if c.ttl <= 0 {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[c.key(token)]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.result, true
}

func (c *cache) put(token string, result *hydra.IntrospectedOAuth2Token) {
	if c.ttl <= 0 || result == nil {
		return
	}

	expiresAt := time.Now().Add(c.ttl)

	// Never outlive the token itself: a result cached past `exp` would keep an
	// expired token working.
	if exp, ok := result.GetExpOk(); ok && *exp > 0 {
		if tokenExpiry := time.Unix(*exp, 0); tokenExpiry.Before(expiresAt) {
			expiresAt = tokenExpiry
		}
	}

	if !expiresAt.After(time.Now()) {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= maxCacheEntries {
		c.evictExpiredLocked()

		// Still full: a flood of distinct tokens. Start over rather than grow.
		if len(c.entries) >= maxCacheEntries {
			c.entries = make(map[string]cacheEntry)
		}
	}

	c.entries[c.key(token)] = cacheEntry{result: result, expiresAt: expiresAt}
}

func (c *cache) evictExpiredLocked() {
	now := time.Now()

	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}
