package grpc

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	fctx "github.com/foundation-go/foundation/context"
	ferr "github.com/foundation-go/foundation/errors"
	fmetrics "github.com/foundation-go/foundation/metrics"
)

// fakeServerStream is the minimum grpc.ServerStream an interceptor needs.
type fakeServerStream struct {
	grpc.ServerStream

	ctx context.Context
}

func (s *fakeServerStream) Context() context.Context { return s.ctx }

func discardLogger() *logrus.Entry {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	return logrus.NewEntry(logger)
}

// Streaming handlers used to see none of the identity the gateway forwards:
// only the unary path copied metadata into the context.
func TestMetadataStreamInterceptorPopulatesTheContext(t *testing.T) {
	userID := uuid.New()

	stream := &fakeServerStream{ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{
		"x-user-id":        {userID.String()},
		"x-authenticated":  {"true"},
		"x-correlation-id": {"corr-1"},
		"x-scope":          {"read write"},
	})}

	var got context.Context
	handler := func(_ interface{}, s grpc.ServerStream) error {
		got = s.Context()
		return nil
	}

	require.NoError(t, MetadataStreamInterceptor(nil, stream, &grpc.StreamServerInfo{}, handler))
	require.NotNil(t, got)

	assert.Equal(t, userID, fctx.GetUserID(got))
	assert.True(t, fctx.GetAuthenticated(got))
	assert.Equal(t, "corr-1", fctx.GetCorrelationID(got))
	assert.True(t, fctx.GetScopes(got).ContainsAll("read", "write"))
}

func TestFoundationErrorToStatusStreamInterceptor(t *testing.T) {
	stream := &fakeServerStream{ctx: context.Background()}

	t.Run("converts a Foundation error", func(t *testing.T) {
		err := FoundationErrorToStatusStreamInterceptor(nil, stream, &grpc.StreamServerInfo{},
			func(interface{}, grpc.ServerStream) error {
				return ferr.NewNotFoundError(nil, "chat", "1")
			})

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
		assert.Equal(t, "not found: chat/1", st.Message())
	})

	t.Run("passes other errors through", func(t *testing.T) {
		sentinel := errors.New("boom")

		err := FoundationErrorToStatusStreamInterceptor(nil, stream, &grpc.StreamServerInfo{},
			func(interface{}, grpc.ServerStream) error { return sentinel })

		assert.ErrorIs(t, err, sentinel)
	})

	t.Run("passes success through", func(t *testing.T) {
		err := FoundationErrorToStatusStreamInterceptor(nil, stream, &grpc.StreamServerInfo{},
			func(interface{}, grpc.ServerStream) error { return nil })

		assert.NoError(t, err)
	})
}

func TestLoggingStreamInterceptorPutsTheLoggerInTheContext(t *testing.T) {
	stream := &fakeServerStream{ctx: fctx.WithCorrelationID(context.Background(), "corr-1")}

	var got *logrus.Entry
	err := LoggingStreamInterceptor(discardLogger())(nil, stream, &grpc.StreamServerInfo{FullMethod: "/svc/Stream"},
		func(_ interface{}, s grpc.ServerStream) error {
			got = fctx.GetLogger(s.Context())
			return nil
		})

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "/svc/Stream", got.Data["method"])
	assert.Equal(t, "corr-1", got.Data["correlation_id"])
}

// grpc-go does not recover handler panics: without an interceptor the whole
// process dies, taking every other in-flight call with it.
func TestRecoveryUnaryInterceptor(t *testing.T) {
	interceptor := RecoveryUnaryInterceptor(discardLogger())

	var (
		resp interface{}
		err  error
	)

	require.NotPanics(t, func() {
		resp, err = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
			func(context.Context, interface{}) (interface{}, error) {
				panic("boom")
			})
	})

	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "internal error", st.Message(), "the panic value must not reach the caller")
}

func TestRecoveryUnaryInterceptorPassesThrough(t *testing.T) {
	interceptor := RecoveryUnaryInterceptor(discardLogger())

	resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(context.Context, interface{}) (interface{}, error) { return "ok", nil })

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestRecoveryStreamInterceptor(t *testing.T) {
	interceptor := RecoveryStreamInterceptor(discardLogger())
	stream := &fakeServerStream{ctx: context.Background()}

	var err error

	require.NotPanics(t, func() {
		err = interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/svc/S"},
			func(interface{}, grpc.ServerStream) error { panic("boom") })
	})

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestRecoveryToleratesANilLogger(t *testing.T) {
	interceptor := RecoveryUnaryInterceptor(nil)

	require.NotPanics(t, func() {
		_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
			func(context.Context, interface{}) (interface{}, error) { panic("boom") })
	})
}

// chainUnary composes interceptors the way grpc.ChainUnaryInterceptor does:
// the first is outermost.
func chainUnary(interceptors []grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		next := handler

		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor, inner := interceptors[i], next
			next = func(ctx context.Context, req interface{}) (interface{}, error) {
				return interceptor(ctx, req, info, inner)
			}
		}

		return next(ctx, req)
	}
}

// A panicking handler must come out of the chain as an Internal status — and it
// must be counted. Metrics used to sit inside recovery, so the unwinding panic
// skipped them: a handler that panicked on every request left the error rate
// flat at zero while the service returned nothing but errors.
func TestDefaultUnaryInterceptorsCountPanickingCalls(t *testing.T) {
	const method = "/svc/PanicMethod"

	before := testutil.ToFloat64(fmetrics.GRPCRequests.WithLabelValues(method, codes.Internal.String()))

	chained := chainUnary(DefaultUnaryInterceptors(discardLogger()))

	resp, err := chained(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: method},
		func(context.Context, interface{}) (interface{}, error) { panic("boom") })

	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())

	after := testutil.ToFloat64(fmetrics.GRPCRequests.WithLabelValues(method, codes.Internal.String()))
	assert.Equal(t, 1.0, after-before, "a panicking call must still be counted")
}

// The status recorded has to be the one the caller receives, i.e. after the
// Foundation error has been converted.
func TestDefaultUnaryInterceptorsRecordTheConvertedStatus(t *testing.T) {
	const method = "/svc/NotFoundMethod"

	before := testutil.ToFloat64(fmetrics.GRPCRequests.WithLabelValues(method, codes.NotFound.String()))

	chained := chainUnary(DefaultUnaryInterceptors(discardLogger()))

	_, err := chained(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: method},
		func(context.Context, interface{}) (interface{}, error) {
			return nil, ferr.NewNotFoundError(nil, "chat", "1")
		})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())

	after := testutil.ToFloat64(fmetrics.GRPCRequests.WithLabelValues(method, codes.NotFound.String()))
	assert.Equal(t, 1.0, after-before)
}

func TestDefaultStreamInterceptorsRecoverAndCount(t *testing.T) {
	const method = "/svc/PanicStream"

	before := testutil.ToFloat64(fmetrics.GRPCRequests.WithLabelValues(method, codes.Internal.String()))

	interceptors := DefaultStreamInterceptors(discardLogger())
	require.NotEmpty(t, interceptors)

	next := grpc.StreamHandler(func(interface{}, grpc.ServerStream) error { panic("boom") })
	info := &grpc.StreamServerInfo{FullMethod: method}

	for i := len(interceptors) - 1; i >= 0; i-- {
		interceptor, inner := interceptors[i], next
		next = func(srv interface{}, stream grpc.ServerStream) error {
			return interceptor(srv, stream, info, inner)
		}
	}

	err := next(nil, &fakeServerStream{ctx: context.Background()})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())

	after := testutil.ToFloat64(fmetrics.GRPCRequests.WithLabelValues(method, codes.Internal.String()))
	assert.Equal(t, 1.0, after-before)
}
