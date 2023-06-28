package grpc

import (
	"context"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// LoggingInterceptor logs all incoming Unary gRPC calls with their request and response.
func LoggingInterceptor(log *logrus.Entry) func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		log = log.WithField("method", info.FullMethod)
		log.Info("Call started")
		log.WithField("request", req).Debug("Request")

		resp, err = handler(ctx, req)

		log.WithField("response", resp).Debug("Response")
		log.Info("Call finished")

		return resp, err
	}
}
