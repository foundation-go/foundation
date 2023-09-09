package grpc

import (
	"context"
	"strings"

	"google.golang.org/grpc/metadata"

	"github.com/google/uuid"
	fctx "github.com/ri-nat/foundation/context"
	fhttp "github.com/ri-nat/foundation/http"
)

// GetMetadata retrieves the value of a specified header from the metadata of a given context.
func GetMetadata(ctx context.Context, name string) (s string) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if p, ok := md[name]; ok {
			s = strings.Join(p, ",")
		}
	}

	return
}

// getCorrelationID returns the correlation ID from the given context.
func getCorrelationID(ctx context.Context) string {
	return GetMetadata(ctx, strings.ToLower(fhttp.HeaderXCorrelationID))
}

// getClientID returns the OAuth client ID from the given context. It assumes that the OAuth client ID is a UUID, and
// returns `uuid.Nil` (`00000000-0000-0000-0000-000000000000`) if the client ID is not a valid UUID.
func getClientID(ctx context.Context) uuid.UUID {
	idStr := GetMetadata(ctx, strings.ToLower(fhttp.HeaderXClientID))
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil
	}

	return id
}

// getScopes retrieves the OAuth scopes from the specified context.
func getScopes(ctx context.Context) fctx.Oauth2Scopes {
	return strings.Split(GetMetadata(ctx, strings.ToLower(fhttp.HeaderXScope)), " ")
}

// getUserID returns the user ID from the given context. It assumes that the user ID is a UUID, and returns
// `uuid.Nil` (`00000000-0000-0000-0000-000000000000`) if the user ID is not a valid UUID.
func getUserID(ctx context.Context) uuid.UUID {
	idStr := GetMetadata(ctx, strings.ToLower(fhttp.HeaderXUserID))
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil
	}

	return id
}

// getAccessToken returns the access token from the specified context.
//
// Any prefix (e.g. `Bearer`) is removed from the token.
func getAccessToken(ctx context.Context) string {
	s := GetMetadata(ctx, strings.ToLower(fhttp.HeaderAuthorization))
	parts := strings.Split(s, " ")

	return parts[len(parts)-1]
}

// getAuthenticated returns the authenticated flag from the specified context.
func getAuthenticated(ctx context.Context) bool {
	return GetMetadata(ctx, strings.ToLower(fhttp.HeaderXAuthenticated)) == "true"
}
