package context

import (
	"context"

	log "github.com/sirupsen/logrus"
)

type CtxKey string

const (
	CtxKeyLogger CtxKey = "logger"
)

// GetCtxLogger returns the logger from the context.
func GetCtxLogger(ctx context.Context) *log.Entry {
	return ctx.Value(CtxKeyLogger).(*log.Entry)
}

// SetCtxLogger sets the logger in the context.
func SetCtxLogger(ctx context.Context, logger *log.Entry) context.Context {
	return context.WithValue(ctx, CtxKeyLogger, logger)
}
