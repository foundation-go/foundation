package context

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"
)

type CtxKey string

const (
	CtxKeyAccessToken   CtxKey = "access_token"
	CtxKeyAuthenticated CtxKey = "authenticated"
	CtxKeyClientID      CtxKey = "client_id"
	CtxKeyCorrelationID CtxKey = "correlation_id"
	CtxKeyLogger        CtxKey = "logger"
	CtxKeyScopes        CtxKey = "scopes"
	CtxKeyTX            CtxKey = "tx"
	CtxKeyUserID        CtxKey = "user_id"
	CtxKeyRequestID     CtxKey = "request_id"
)

// Oauth2Scopes represents a list of OAuth scopes.
type Oauth2Scopes []string

// ContainsAll checks if the list of OAuth scopes contains **all** the specified scopes.
func (s Oauth2Scopes) ContainsAll(scopes ...string) bool {
	for _, scope := range scopes {
		found := false

		for _, sc := range s {
			if sc == scope {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

// ContainsAny checks if the list of OAuth scopes contains any of the specified scopes.
func (s Oauth2Scopes) ContainsAny(scopes ...string) bool {
	for _, scope := range scopes {
		for _, sc := range s {
			if sc == scope {
				return true
			}
		}
	}

	return false
}

// lookup reads a typed value from the context. It never panics: a missing key
// and a key holding a value of a different type are both reported as absent.
func lookup[T any](ctx context.Context, key CtxKey) (T, bool) {
	if ctx == nil {
		var zero T
		return zero, false
	}

	value, ok := ctx.Value(key).(T)

	return value, ok
}

// get reads a typed value from the context, falling back to the zero value.
func get[T any](ctx context.Context, key CtxKey) T {
	value, _ := lookup[T](ctx, key)

	return value
}

// mustGet reads a typed value from the context and panics with a descriptive
// message if it is missing.
func mustGet[T any](ctx context.Context, key CtxKey) T {
	value, ok := lookup[T](ctx, key)
	if !ok {
		panic(fmt.Sprintf("foundation/context: `%s` is not set in the context", key))
	}

	return value
}

//
// Correlation ID
//

// GetCorrelationID returns the correlation ID from the context, or an empty
// string if the context does not carry one.
func GetCorrelationID(ctx context.Context) string {
	return get[string](ctx, CtxKeyCorrelationID)
}

// LookupCorrelationID returns the correlation ID from the context and reports
// whether it was present.
func LookupCorrelationID(ctx context.Context) (string, bool) {
	return lookup[string](ctx, CtxKeyCorrelationID)
}

// MustGetCorrelationID returns the correlation ID from the context and panics
// if it is not set.
func MustGetCorrelationID(ctx context.Context) string {
	return mustGet[string](ctx, CtxKeyCorrelationID)
}

// WithCorrelationID sets the correlation ID to the context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CtxKeyCorrelationID, correlationID)
}

//
// OAuth client ID
//

// GetClientID returns the OAuth client ID from the context, or `uuid.Nil` if
// the context does not carry one.
func GetClientID(ctx context.Context) uuid.UUID {
	return get[uuid.UUID](ctx, CtxKeyClientID)
}

// LookupClientID returns the OAuth client ID from the context and reports
// whether it was present.
func LookupClientID(ctx context.Context) (uuid.UUID, bool) {
	return lookup[uuid.UUID](ctx, CtxKeyClientID)
}

// MustGetClientID returns the OAuth client ID from the context and panics if it
// is not set.
func MustGetClientID(ctx context.Context) uuid.UUID {
	return mustGet[uuid.UUID](ctx, CtxKeyClientID)
}

// WithClientID sets the OAuth client ID to the context
func WithClientID(ctx context.Context, clientID uuid.UUID) context.Context {
	return context.WithValue(ctx, CtxKeyClientID, clientID)
}

//
// User ID
//

// GetUserID returns the user ID from the context, or `uuid.Nil` if the context
// does not carry one.
func GetUserID(ctx context.Context) uuid.UUID {
	return get[uuid.UUID](ctx, CtxKeyUserID)
}

// LookupUserID returns the user ID from the context and reports whether it was
// present.
func LookupUserID(ctx context.Context) (uuid.UUID, bool) {
	return lookup[uuid.UUID](ctx, CtxKeyUserID)
}

// MustGetUserID returns the user ID from the context and panics if it is not
// set.
func MustGetUserID(ctx context.Context) uuid.UUID {
	return mustGet[uuid.UUID](ctx, CtxKeyUserID)
}

// WithUserID sets the user ID to the context
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, CtxKeyUserID, userID)
}

//
// Access token
//

// GetAccessToken returns the access token from the context, or an empty string
// if the context does not carry one.
func GetAccessToken(ctx context.Context) string {
	return get[string](ctx, CtxKeyAccessToken)
}

// LookupAccessToken returns the access token from the context and reports
// whether it was present.
func LookupAccessToken(ctx context.Context) (string, bool) {
	return lookup[string](ctx, CtxKeyAccessToken)
}

// WithAccessToken sets the access token to the context
func WithAccessToken(ctx context.Context, accessToken string) context.Context {
	return context.WithValue(ctx, CtxKeyAccessToken, accessToken)
}

//
// Authenticated flag
//

// GetAuthenticated returns the authenticated flag from the context, or false if
// the context does not carry one.
func GetAuthenticated(ctx context.Context) bool {
	return get[bool](ctx, CtxKeyAuthenticated)
}

// LookupAuthenticated returns the authenticated flag from the context and
// reports whether it was present.
func LookupAuthenticated(ctx context.Context) (bool, bool) {
	return lookup[bool](ctx, CtxKeyAuthenticated)
}

// WithAuthenticated sets the authenticated flag to the context
func WithAuthenticated(ctx context.Context, authenticated bool) context.Context {
	return context.WithValue(ctx, CtxKeyAuthenticated, authenticated)
}

//
// Logger
//

var (
	fallbackLoggerOnce sync.Once
	fallbackLogger     *log.Entry
)

// FallbackLogger returns the logger used by GetLogger when the context does not
// carry one. It is a plain logrus entry tagged so that such call sites are
// identifiable in the logs.
func FallbackLogger() *log.Entry {
	fallbackLoggerOnce.Do(func() {
		fallbackLogger = log.NewEntry(log.StandardLogger()).WithField("logger", "fallback")
	})

	return fallbackLogger
}

// GetLogger returns the logger from the context. If the context does not carry
// a logger, a fallback logger is returned so that call sites never have to
// nil-check the result.
func GetLogger(ctx context.Context) *log.Entry {
	logger, ok := lookup[*log.Entry](ctx, CtxKeyLogger)
	if !ok || logger == nil {
		return FallbackLogger()
	}

	return logger
}

// LookupLogger returns the logger from the context and reports whether it was
// present.
func LookupLogger(ctx context.Context) (*log.Entry, bool) {
	logger, ok := lookup[*log.Entry](ctx, CtxKeyLogger)
	if !ok || logger == nil {
		return nil, false
	}

	return logger, true
}

// WithLogger sets the logger to the context
func WithLogger(ctx context.Context, logger *log.Entry) context.Context {
	return context.WithValue(ctx, CtxKeyLogger, logger)
}

//
// OAuth scopes
//

// GetScopes returns the OAuth scopes from the context, or nil if the context
// does not carry any.
func GetScopes(ctx context.Context) Oauth2Scopes {
	return get[Oauth2Scopes](ctx, CtxKeyScopes)
}

// LookupScopes returns the OAuth scopes from the context and reports whether
// they were present.
func LookupScopes(ctx context.Context) (Oauth2Scopes, bool) {
	return lookup[Oauth2Scopes](ctx, CtxKeyScopes)
}

// WithScopes sets the OAuth scopes to the context
func WithScopes(ctx context.Context, scopes Oauth2Scopes) context.Context {
	return context.WithValue(ctx, CtxKeyScopes, scopes)
}

//
// Database transaction
//

// GetTX returns the transaction from the context, or nil if the context does
// not carry one. Use LookupTX or MustGetTX when the absence of a transaction is
// a programming error.
func GetTX(ctx context.Context) pgx.Tx {
	return get[pgx.Tx](ctx, CtxKeyTX)
}

// LookupTX returns the transaction from the context and reports whether it was
// present.
func LookupTX(ctx context.Context) (pgx.Tx, bool) {
	return lookup[pgx.Tx](ctx, CtxKeyTX)
}

// MustGetTX returns the transaction from the context and panics if it is not
// set.
func MustGetTX(ctx context.Context) pgx.Tx {
	return mustGet[pgx.Tx](ctx, CtxKeyTX)
}

// WithTX sets the transaction to the context
func WithTX(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, CtxKeyTX, tx)
}

//
// Request ID
//

// GetRequestID returns the request ID from the context, or an empty string if
// the context does not carry one.
func GetRequestID(ctx context.Context) string {
	return get[string](ctx, CtxKeyRequestID)
}

// LookupRequestID returns the request ID from the context and reports whether
// it was present.
func LookupRequestID(ctx context.Context) (string, bool) {
	return lookup[string](ctx, CtxKeyRequestID)
}

// WithRequestID sets the request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, CtxKeyRequestID, requestID)
}
