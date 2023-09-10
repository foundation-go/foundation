package grpc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	fctx "github.com/ri-nat/foundation/context"
	fhttp "github.com/ri-nat/foundation/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// MetadataUnaryInterceptor sets values passed by the gateway from the gRPC metadata into the context.
//
// This is implemented to standardize the retrieval of common values from the context, regardless of whether
// we are writing gRPC or Event Bus handlers.
func MetadataUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	ctx = fctx.WithCorrelationID(ctx, getCorrelationID(ctx))
	ctx = fctx.WithClientID(ctx, getClientID(ctx))
	ctx = fctx.WithScopes(ctx, getScopes(ctx))
	ctx = fctx.WithUserID(ctx, getUserID(ctx))
	ctx = fctx.WithAccessToken(ctx, getAccessToken(ctx))
	ctx = fctx.WithAuthenticated(ctx, getAuthenticated(ctx))

	resp, err = handler(ctx, req)

	return resp, err
}

// GetMetadataValue retrieves the value of a specified key from the metadata of a gRPC context.
func GetMetadataValue(ctx context.Context, key string) (s string) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if p, ok := md[key]; ok {
			s = strings.Join(p, ",")
		}
	}

	return
}

// getCorrelationID returns the correlation ID from the specified gRPC context.
func getCorrelationID(ctx context.Context) string {
	return GetMetadataValue(ctx, strings.ToLower(fhttp.HeaderXCorrelationID))
}

// getClientID returns the OAuth client ID from the specified gRPC context. It assumes that the OAuth client ID is a UUID, and
// returns `uuid.Nil` (`00000000-0000-0000-0000-000000000000`) if the client ID is not a valid UUID.
func getClientID(ctx context.Context) uuid.UUID {
	idStr := GetMetadataValue(ctx, strings.ToLower(fhttp.HeaderXClientID))
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil
	}

	return id
}

// getScopes retrieves the OAuth scopes from the specified gRPC context.
func getScopes(ctx context.Context) fctx.Oauth2Scopes {
	return strings.Split(GetMetadataValue(ctx, strings.ToLower(fhttp.HeaderXScope)), " ")
}

// getUserID returns the user ID from the given context. It assumes that the user ID is a UUID, and returns
// `uuid.Nil` (`00000000-0000-0000-0000-000000000000`) if the user ID is not a valid UUID.
func getUserID(ctx context.Context) uuid.UUID {
	idStr := GetMetadataValue(ctx, strings.ToLower(fhttp.HeaderXUserID))
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil
	}

	return id
}

// getAccessToken returns the access token from the specified gRPC context.
//
// Any prefix (e.g. `Bearer`) is removed from the token.
func getAccessToken(ctx context.Context) string {
	s := GetMetadataValue(ctx, strings.ToLower(fhttp.HeaderAuthorization))
	parts := strings.Split(s, " ")

	return parts[len(parts)-1]
}

// getAuthenticated returns the authenticated flag from the specified gRPC context.
func getAuthenticated(ctx context.Context) bool {
	return GetMetadataValue(ctx, strings.ToLower(fhttp.HeaderXAuthenticated)) == "true"
}
