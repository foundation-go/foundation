package context

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type CtxKey string

const (
	CtxKeyAccessToken   CtxKey = "access_token"
	CtxKeyAuthenticated CtxKey = "authenticated"
	CtxKeyClientID      CtxKey = "client_id"
	CtxKeyCorrelationID CtxKey = "correlation_id"
	CtxKeyLogger        CtxKey = "logger"
	CtxKeyTX            CtxKey = "tx"
	CtxKeyUserID        CtxKey = "user_id"
)

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

// GetTX returns the transaction from the context.
func GetTX(ctx context.Context) *sql.Tx {
	return ctx.Value(CtxKeyTX).(*sql.Tx)
}

// WithTX sets the transaction to the context
func WithTX(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, CtxKeyTX, tx)
}
