package grpc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	fctx "github.com/foundation-go/foundation/context"
	fhttp "github.com/foundation-go/foundation/http"
)

// MetadataUnaryInterceptor sets values passed by the gateway from the gRPC metadata into the context.
//
// This is implemented to standardize the retrieval of common values from the context, regardless of whether
// we are writing gRPC or Event Bus handlers.
func MetadataUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	return handler(contextFromMetadata(ctx), req)
}

// contextFromMetadata copies the values the gateway forwards as gRPC metadata
// into the context, so that handlers read them the same way regardless of
// whether they were reached over gRPC or through the event bus.
func contextFromMetadata(ctx context.Context) context.Context {
	ctx = fctx.WithCorrelationID(ctx, getCorrelationID(ctx))
	ctx = fctx.WithClientID(ctx, getClientID(ctx))
	ctx = fctx.WithScopes(ctx, getScopes(ctx))
	ctx = fctx.WithUserID(ctx, getUserID(ctx))
	ctx = fctx.WithAccessToken(ctx, getAccessToken(ctx))
	ctx = fctx.WithAuthenticated(ctx, getAuthenticated(ctx))
	ctx = fctx.WithRequestID(ctx, getRequestID(ctx))

	return ctx
}

// GetMetadataValue retrieves the value of a specified key from the metadata of a gRPC context.
//
// When a key carries several values only the first is returned. Joining them —
// as this used to do, with a comma — produces a string that is not a valid
// value of anything: a duplicated `x-user-id` became `id1,id2`, and a
// duplicated `x-scope` became `scopeA,scopeB scopeC`, which then split on
// whitespace into scopes that were never granted.
func GetMetadataValue(ctx context.Context, key string) string {
	values := GetMetadataValues(ctx, key)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

// GetMetadataValues retrieves every value stored under a key in the metadata of
// a gRPC context.
func GetMetadataValues(ctx context.Context, key string) []string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}

	return md[key]
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

// getRequestID returns the request ID from the specified gRPC context.
func getRequestID(ctx context.Context) string {
	return GetMetadataValue(ctx, strings.ToLower(fhttp.HeaderXRequestID))
}
