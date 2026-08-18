package grpc

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	fmetrics "github.com/foundation-go/foundation/metrics"
)

// RecoveryUnaryInterceptor turns a panic in a handler into an `Internal` status,
// a log line carrying the stack, and a Sentry report.
//
// grpc-go does not recover handler panics: without this the whole process dies,
// taking every other in-flight call with it.
func RecoveryUnaryInterceptor(log *logrus.Entry) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = handlePanic(log, info.FullMethod, recovered)
				resp = nil
			}
		}()

		return handler(ctx, req)
	}
}

// RecoveryStreamInterceptor is the streaming counterpart of
// RecoveryUnaryInterceptor.
func RecoveryStreamInterceptor(log *logrus.Entry) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = handlePanic(log, info.FullMethod, recovered)
			}
		}()

		return handler(srv, stream)
	}
}

// handlePanic reports a recovered panic and produces the status returned to the
// caller. The panic value itself is deliberately not sent to the client.
func handlePanic(log *logrus.Entry, method string, recovered interface{}) error {
	err := fmt.Errorf("panic in %s: %v", method, recovered)

	if log == nil {
		log = logrus.NewEntry(logrus.StandardLogger())
	}

	log.WithField("method", method).
		WithField("stack", string(debug.Stack())).
		WithError(err).
		Error("Recovered from a panic in a gRPC handler")

	fmetrics.Panics.WithLabelValues("grpc").Inc()
	sentry.CaptureException(err)

	return status.Error(codes.Internal, "internal error")
}
