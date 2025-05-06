package context

import (
	"context"

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

// GetCorrelationID returns the correlation ID from the context.
func GetCorrelationID(ctx context.Context) string {
	return ctx.Value(CtxKeyCorrelationID).(string)
}

// WithCorrelationID sets the correlation ID to the context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CtxKeyCorrelationID, correlationID)
}

// GetClientID returns the OAuth client ID from the context.
func GetClientID(ctx context.Context) uuid.UUID {
	return ctx.Value(CtxKeyClientID).(uuid.UUID)
}

// WithClientID sets the OAuth client ID to the context
func WithClientID(ctx context.Context, clientID uuid.UUID) context.Context {
	return context.WithValue(ctx, CtxKeyClientID, clientID)
}

// GetUserID returns the user ID from the context.
func GetUserID(ctx context.Context) uuid.UUID {
	return ctx.Value(CtxKeyUserID).(uuid.UUID)
}

// WithUserID sets the user ID to the context
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, CtxKeyUserID, userID)
}

// GetAccessToken returns the access token from the context.
func GetAccessToken(ctx context.Context) string {
	return ctx.Value(CtxKeyAccessToken).(string)
}

// WithAccessToken sets the access token to the context
func WithAccessToken(ctx context.Context, accessToken string) context.Context {
	return context.WithValue(ctx, CtxKeyAccessToken, accessToken)
}

// GetAuthenticated returns the authenticated flag from the context.
func GetAuthenticated(ctx context.Context) bool {
	return ctx.Value(CtxKeyAuthenticated).(bool)
}

// WithAuthenticated sets the authenticated flag to the context
func WithAuthenticated(ctx context.Context, authenticated bool) context.Context {
	return context.WithValue(ctx, CtxKeyAuthenticated, authenticated)
}

// GetLogger returns the logger from the context.
func GetLogger(ctx context.Context) *log.Entry {
	return ctx.Value(CtxKeyLogger).(*log.Entry)
}

// WithLogger sets the logger to the context
func WithLogger(ctx context.Context, logger *log.Entry) context.Context {
	return context.WithValue(ctx, CtxKeyLogger, logger)
}

// GetScopes returns the OAuth scopes from the context.
func GetScopes(ctx context.Context) Oauth2Scopes {
	return ctx.Value(CtxKeyScopes).(Oauth2Scopes)
}

// WithScopes sets the OAuth scopes to the context
func WithScopes(ctx context.Context, scopes Oauth2Scopes) context.Context {
	return context.WithValue(ctx, CtxKeyScopes, scopes)
}

// GetTX returns the transaction from the context.
func GetTX(ctx context.Context) pgx.Tx {
	return ctx.Value(CtxKeyTX).(pgx.Tx)
}

// WithTX sets the transaction to the context
func WithTX(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, CtxKeyTX, tx)
}

// GetRequestID returns the request ID from the context.
func GetRequestID(ctx context.Context) string {
	return ctx.Value(CtxKeyRequestID).(string)
}

// WithRequestID sets the request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, CtxKeyRequestID, requestID)
}
