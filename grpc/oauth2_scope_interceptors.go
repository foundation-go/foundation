package grpc

import (
	"context"

	"google.golang.org/grpc"

	fctx "github.com/ri-nat/foundation/context"
	ferr "github.com/ri-nat/foundation/errors"
)

// CheckAllScopesPresenceUnaryInterceptor checks if the context contains all specified scopes.
func CheckAllScopesPresenceUnaryInterceptor(scopes ...string) func(context.Context, interface{}, *grpc.UnaryServerInfo, grpc.UnaryHandler) (interface{}, error) {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		requestScopes := fctx.GetScopes(ctx)

		// Check if the request contains all required scopes
		if !requestScopes.ContainsAll(scopes...) {
			return nil, ferr.NewInsufficientScopeAllError(scopes...)
		}

		// Call handler
		return handler(ctx, req)
	}
}

// CheckAnyScopePresenceUnaryInterceptor checks if the context contains at least one of the specified scopes.
func CheckAnyScopePresenceUnaryInterceptor(scopes ...string) func(context.Context, interface{}, *grpc.UnaryServerInfo, grpc.UnaryHandler) (interface{}, error) {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		requestScopes := fctx.GetScopes(ctx)

		// Check if the request contains at least one of the required scopes
		if !requestScopes.ContainsAny(scopes...) {
			return nil, ferr.NewInsufficientScopeAnyError(scopes...)
		}

		// Call handler
		return handler(ctx, req)
	}
}
