package context

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

type CtxKey string

const (
	CtxKeyAccessToken   CtxKey = "access_token"
	CtxKeyAuthenticated CtxKey = "authenticated"
	CtxKeyClientID      CtxKey = "client_id"
	CtxKeyCorrelationID CtxKey = "correlation_id"
	CtxKeyLogger        CtxKey = "logger"
	CtxKeyRedis         CtxKey = "redis"
	CtxKeyTX            CtxKey = "tx"
	CtxKeyUserID        CtxKey = "user_id"
)

// GetCorrelationID returns the correlation ID from the context.
func GetCorrelationID(ctx context.Context) string {
	return ctx.Value(CtxKeyCorrelationID).(string)
}

// SetCorrelationID sets the correlation ID in the context.
func SetCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CtxKeyCorrelationID, correlationID)
}

// GetClientID returns the OAuth client ID from the context.
func GetClientID(ctx context.Context) uuid.UUID {
	return ctx.Value(CtxKeyClientID).(uuid.UUID)
}

// SetClientID sets the OAuth client ID in the context.
func SetClientID(ctx context.Context, clientID uuid.UUID) context.Context {
	return context.WithValue(ctx, CtxKeyClientID, clientID)
}

// GetUserID returns the user ID from the context.
func GetUserID(ctx context.Context) uuid.UUID {
	return ctx.Value(CtxKeyUserID).(uuid.UUID)
}

// SetUserID sets the user ID in the context.
func SetUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, CtxKeyUserID, userID)
}

// GetAccessToken returns the access token from the context.
func GetAccessToken(ctx context.Context) string {
	return ctx.Value(CtxKeyAccessToken).(string)
}

// SetAccessToken sets the access token in the context.
func SetAccessToken(ctx context.Context, accessToken string) context.Context {
	return context.WithValue(ctx, CtxKeyAccessToken, accessToken)
}

// GetAuthenticated returns the authenticated flag from the context.
func GetAuthenticated(ctx context.Context) bool {
	return ctx.Value(CtxKeyAuthenticated).(bool)
}

// SetAuthenticated sets the authenticated flag in the context.
func SetAuthenticated(ctx context.Context, authenticated bool) context.Context {
	return context.WithValue(ctx, CtxKeyAuthenticated, authenticated)
}

// GetLogger returns the logger from the context.
func GetLogger(ctx context.Context) *log.Entry {
	return ctx.Value(CtxKeyLogger).(*log.Entry)
}

// SetLogger sets the logger in the context.
func SetLogger(ctx context.Context, logger *log.Entry) context.Context {
	return context.WithValue(ctx, CtxKeyLogger, logger)
}

// GetTX returns the transaction from the context.
func GetTX(ctx context.Context) *sql.Tx {
	return ctx.Value(CtxKeyTX).(*sql.Tx)
}

// SetTX sets the transaction in the context.
func SetTX(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, CtxKeyTX, tx)
}

// GetRedis returns the redis client from the context.
func GetRedis(ctx context.Context) *redis.Client {
	return ctx.Value(CtxKeyRedis).(*redis.Client)
}

// SetRedis sets the redis client in the context.
func SetRedis(ctx context.Context, redis *redis.Client) context.Context {
	return context.WithValue(ctx, CtxKeyRedis, redis)
}
