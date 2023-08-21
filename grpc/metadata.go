package grpc

import (
	"context"

	fctx "github.com/ri-nat/foundation/context"
	"google.golang.org/grpc"
)

// MetadataInterceptor sets the correlation ID, user ID and access token from the gRPC metadata to the context.
//
// We're doing this because we want to unify the way we get common values from the context no matter if we're writing
// gRPC or Event Bus handlers.
//
// TODO: Make `fctx.Get*` functions clever enough to get the values from the gRPC metadata if the context is a gRPC
// context and get rid of this interceptor.
func MetadataInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	ctx = fctx.WithCorrelationID(ctx, GetCorrelationID(ctx))
	ctx = fctx.WithClientID(ctx, GetClientID(ctx))
	ctx = fctx.WithUserID(ctx, GetUserID(ctx))
	ctx = fctx.WithAccessToken(ctx, GetAccessToken(ctx))
	ctx = fctx.WithAuthenticated(ctx, GetAuthenticated(ctx))

	resp, err = handler(ctx, req)

	return resp, err
}
