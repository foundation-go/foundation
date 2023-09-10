package grpc

import (
	"context"

	"google.golang.org/grpc"

	fctx "github.com/ri-nat/foundation/context"
)

// CheckAllScopesPresenceUnaryInterceptor checks if the context contains all specified scopes.
func CheckAllScopesPresenceUnaryInterceptor(scopes ...string) func(context.Context, interface{}, *grpc.UnaryServerInfo, grpc.UnaryHandler) (interface{}, error) {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := fctx.CheckAllScopesPresence(ctx, scopes...); err != nil {
			return nil, err
		}

		// Call handler
		return handler(ctx, req)
	}
}

// CheckAnyScopePresenceUnaryInterceptor checks if the context contains at least one of the specified scopes.
func CheckAnyScopePresenceUnaryInterceptor(scopes ...string) func(context.Context, interface{}, *grpc.UnaryServerInfo, grpc.UnaryHandler) (interface{}, error) {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := fctx.CheckAnyScopePresence(ctx, scopes...); err != nil {
			return nil, err
		}

		// Call handler
		return handler(ctx, req)
	}
}
