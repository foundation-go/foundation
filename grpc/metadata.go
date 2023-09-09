package grpc

import (
	"context"

	fctx "github.com/ri-nat/foundation/context"
	"google.golang.org/grpc"
)

// MetadataInterceptor sets values passed by the gateway from the gRPC metadata into the context.
//
// This is implemented to standardize the retrieval of common values from the context, regardless of whether
// we are writing gRPC or Event Bus handlers.
//
// TODO: Enhance the `fctx.Get*` functions to fetch values from the gRPC metadata when the context is a gRPC
// context, eliminating the need for this interceptor.
func MetadataInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	ctx = fctx.WithCorrelationID(ctx, getCorrelationID(ctx))
	ctx = fctx.WithClientID(ctx, getClientID(ctx))
	ctx = fctx.WithScopes(ctx, getScopes(ctx))
	ctx = fctx.WithUserID(ctx, getUserID(ctx))
	ctx = fctx.WithAccessToken(ctx, getAccessToken(ctx))
	ctx = fctx.WithAuthenticated(ctx, getAuthenticated(ctx))

	resp, err = handler(ctx, req)

	return resp, err
}
