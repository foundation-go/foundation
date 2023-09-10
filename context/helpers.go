package context

import (
	"context"

	ferr "github.com/ri-nat/foundation/errors"
)

// CheckAllScopesPresence checks if the context contains all specified scopes.
// If any of the specified scopes is missing, it returns an "insufficient scope" error.
func CheckAllScopesPresence(ctx context.Context, scopes ...string) *ferr.PermissionDeniedError {
	if !GetScopes(ctx).ContainsAll(scopes...) {
		return ferr.NewInsufficientScopeAllError(scopes...)
	}

	return nil
}

// CheckAnyScopePresence checks if the context contains at least one of the specified scopes.
// If none of the specified scopes are present, it returns an "insufficient scope" error.
func CheckAnyScopePresence(ctx context.Context, scopes ...string) *ferr.PermissionDeniedError {
	if !GetScopes(ctx).ContainsAny(scopes...) {
		return ferr.NewInsufficientScopeAnyError(scopes...)
	}

	return nil
}
