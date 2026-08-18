package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	fmetrics "github.com/foundation-go/foundation/metrics"
)

// MetricsUnaryInterceptor records the rate, status distribution and duration of
// gRPC calls.
func MetricsUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	started := time.Now()

	resp, err := handler(ctx, req)

	observeGRPCCall(info.FullMethod, err, started)

	return resp, err
}

// MetricsStreamInterceptor is the streaming counterpart of
// MetricsUnaryInterceptor. It measures the lifetime of the stream.
func MetricsStreamInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	started := time.Now()

	err := handler(srv, stream)

	observeGRPCCall(info.FullMethod, err, started)

	return err
}

// observeGRPCCall records one finished call.
//
// The status code is taken with status.Code, which maps a plain error to
// Unknown and a nil error to OK, so the label is always one of the gRPC codes
// rather than free-form text.
func observeGRPCCall(method string, err error, started time.Time) {
	fmetrics.GRPCRequests.WithLabelValues(method, status.Code(err).String()).Inc()
	fmetrics.GRPCRequestDuration.WithLabelValues(method).Observe(time.Since(started).Seconds())
}
