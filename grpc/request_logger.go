package grpc

import (
	"context"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	fctx "github.com/ri-nat/foundation/context"
)

// LoggingInterceptor returns a gRPC unary interceptor that logs all incoming gRPC calls.
// It logs the method details, request, response, and any potential errors.
func LoggingInterceptor(log *logrus.Entry) func(context.Context, interface{}, *grpc.UnaryServerInfo, grpc.UnaryHandler) (interface{}, error) {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		// Enhance the log with request-related fields.
		log = log.WithFields(logrus.Fields{
			"method":         info.FullMethod,
			"correlation_id": fctx.GetCorrelationID(ctx),
		})

		log.Info("Call started")
		log.WithField("request", req).Debug("Request")

		// Add logger to context
		ctx = fctx.WithLogger(ctx, log)

		// Call handler
		resp, err = handler(ctx, req)

		// Process handling error if any
		if err != nil {
			log.WithError(err).Error("Call failed")
			return nil, err
		}

		log.WithField("response", resp).Debug("Response")
		log.Info("Call finished")

		return resp, nil
	}
}
