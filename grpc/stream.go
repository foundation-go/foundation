package grpc

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	fctx "github.com/foundation-go/foundation/context"
	ferr "github.com/foundation-go/foundation/errors"
)

// wrappedServerStream lets an interceptor replace the context a streaming
// handler sees. grpc.ServerStream exposes its context read-only, so the only
// way to enrich it is to wrap the stream.
type wrappedServerStream struct {
	grpc.ServerStream

	ctx context.Context
}

func (s *wrappedServerStream) Context() context.Context { return s.ctx }

// withContext wraps stream so that handlers observe ctx.
func withContext(stream grpc.ServerStream, ctx context.Context) grpc.ServerStream {
	return &wrappedServerStream{ServerStream: stream, ctx: ctx}
}

// MetadataStreamInterceptor is the streaming counterpart of
// MetadataUnaryInterceptor.
//
// Without it, streaming handlers see none of the identity the gateway
// forwards — `fctx.GetUserID` returns uuid.Nil, and the scope interceptors have
// nothing to check.
func MetadataStreamInterceptor(srv interface{}, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return handler(srv, withContext(stream, contextFromMetadata(stream.Context())))
}

// FoundationErrorToStatusStreamInterceptor is the streaming counterpart of
// FoundationErrorToStatusUnaryInterceptor.
func FoundationErrorToStatusStreamInterceptor(srv interface{}, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := handler(srv, stream)
	if err == nil {
		return nil
	}

	var fErr ferr.FoundationError
	if errors.As(err, &fErr) {
		return fErr.GRPCStatus().Err()
	}

	return err
}

// LoggingStreamInterceptor is the streaming counterpart of
// LoggingUnaryInterceptor.
func LoggingStreamInterceptor(log *logrus.Entry, _ ...LoggingOption) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := stream.Context()

		// N.B.: a local entry, never an assignment back to `log` — that
		// variable is shared by every concurrent stream.
		callLog := log.WithFields(logrus.Fields{
			"method":         info.FullMethod,
			"correlation_id": fctx.GetCorrelationID(ctx),
		})

		callLog.Info("Stream started")

		err := handler(srv, withContext(stream, fctx.WithLogger(ctx, callLog)))
		if err != nil {
			callLog.WithError(err).Error("Stream failed")

			return err
		}

		callLog.Info("Stream finished")

		return nil
	}
}

// DefaultStreamInterceptors returns the stream interceptors Foundation installs
// on every gRPC server, in the order they run.
func DefaultStreamInterceptors(log *logrus.Entry, opts ...LoggingOption) []grpc.StreamServerInterceptor {
	return []grpc.StreamServerInterceptor{
		// Metrics are outermost: outside the error conversion, so the status
		// code recorded is the one the caller receives, and outside recovery,
		// so a call that panics is still counted. Inside recovery it was
		// skipped entirely by the unwinding panic, and a handler that panicked
		// on every request left the error rate flat at zero.
		MetricsStreamInterceptor,
		RecoveryStreamInterceptor(log),
		MetadataStreamInterceptor,
		FoundationErrorToStatusStreamInterceptor,
		LoggingStreamInterceptor(log, opts...),
	}
}

// DefaultUnaryInterceptors returns the unary interceptors Foundation installs
// on every gRPC server, in the order they run.
func DefaultUnaryInterceptors(log *logrus.Entry, opts ...LoggingOption) []grpc.UnaryServerInterceptor {
	return []grpc.UnaryServerInterceptor{
		// Metrics are outermost: outside the error conversion, so the status
		// code recorded is the one the caller receives, and outside recovery,
		// so a call that panics is still counted. Inside recovery it was
		// skipped entirely by the unwinding panic, and a handler that panicked
		// on every request left the error rate flat at zero.
		MetricsUnaryInterceptor,
		// Recovery covers every interceptor below it.
		RecoveryUnaryInterceptor(log),
		MetadataUnaryInterceptor,
		FoundationErrorToStatusUnaryInterceptor,
		LoggingUnaryInterceptor(log, opts...),
	}
}
