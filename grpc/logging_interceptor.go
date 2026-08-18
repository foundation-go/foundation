package grpc

import (
	"context"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	fctx "github.com/foundation-go/foundation/context"
)

// loggingOptions configures the logging interceptors.
type loggingOptions struct {
	logPayloads bool
}

// LoggingOption configures a Foundation logging interceptor.
type LoggingOption func(*loggingOptions)

// WithPayloadLogging enables logging the full request and response of every
// call at Debug level.
//
// It is off by default. A protobuf message printed whole contains whatever the
// caller sent — passwords, tokens, personal data — and `LOG_LEVEL=debug` on a
// production service is a routine thing to do while investigating something
// unrelated. Turn it on deliberately, in an environment where that is
// acceptable.
func WithPayloadLogging(enabled bool) LoggingOption {
	return func(o *loggingOptions) {
		o.logPayloads = enabled
	}
}

func newLoggingOptions(opts []LoggingOption) *loggingOptions {
	options := &loggingOptions{}

	for _, opt := range opts {
		opt(options)
	}

	return options
}

// LoggingUnaryInterceptor returns a gRPC unary interceptor that logs all incoming gRPC calls.
// It logs the method details and any potential errors; request and response
// bodies are only logged when WithPayloadLogging is set.
func LoggingUnaryInterceptor(log *logrus.Entry, opts ...LoggingOption) grpc.UnaryServerInterceptor {
	options := newLoggingOptions(opts)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		// Enhance the log with request-related fields.
		//
		// N.B.: this MUST NOT assign back to `log` — that variable is captured
		// by the closure and shared by every in-flight call, so reassigning it
		// is a data race and leaks fields between concurrent calls.
		callLog := log.WithFields(logrus.Fields{
			"method":         info.FullMethod,
			"correlation_id": fctx.GetCorrelationID(ctx),
		})

		callLog.Info("Call started")

		if options.logPayloads {
			callLog.WithField("request", req).Debug("Request")
		}

		// Add logger to context
		ctx = fctx.WithLogger(ctx, callLog)

		// Call handler
		resp, err = handler(ctx, req)

		// Process handling error if any
		if err != nil {
			callLog.WithError(err).Error("Call failed")

			return nil, err
		}

		if options.logPayloads {
			callLog.WithField("response", resp).Debug("Response")
		}

		callLog.Info("Call finished")

		return resp, nil
	}
}
