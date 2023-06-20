package context

import (
	"context"
	"database/sql"

	log "github.com/sirupsen/logrus"
)

type CtxKey string

const (
	CtxKeyCorrelationID CtxKey = "correlation_id"
	CtxKeyLogger        CtxKey = "logger"
	CtxKeyTX            CtxKey = "tx"
)

// GetCorrelationID returns the correlation ID from the context.
func GetCorrelationID(ctx context.Context) string {
	return ctx.Value(CtxKeyCorrelationID).(string)
}

// SetCorrelationID sets the correlation ID in the context.
func SetCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CtxKeyCorrelationID, correlationID)
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
