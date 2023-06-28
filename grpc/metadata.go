package grpc

import (
	"context"

	fctx "github.com/ri-nat/foundation/context"
	"google.golang.org/grpc"
)

// MetadataInterceptor sets the correlation ID, user ID and access token from the gRPC metadata to the context.
func MetadataInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	ctx = fctx.SetCorrelationID(ctx, GetCorrelationID(ctx))
	ctx = fctx.SetUserID(ctx, GetUserID(ctx))
	ctx = fctx.SetAccessToken(ctx, GetAccessToken(ctx))
	ctx = fctx.SetAuthenticated(ctx, GetAuthenticated(ctx))

	resp, err = handler(ctx, req)

	return resp, err
}
