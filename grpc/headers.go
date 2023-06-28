package grpc

import (
	"context"
	"strings"

	"google.golang.org/grpc/metadata"

	"github.com/google/uuid"
	fhttp "github.com/ri-nat/foundation/http"
)

// GetHeader returns the value of the given header from the given context.
func GetHeader(ctx context.Context, name string) (s string) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if p, ok := md[name]; ok {
			s = strings.Join(p, ",")
		}
	}

	return
}

// GetCorrelationID returns the correlation ID from the given context.
func GetCorrelationID(ctx context.Context) string {
	return GetHeader(ctx, strings.ToLower(fhttp.HeaderXCorrelationID))
}

// GetUserID returns the user ID from the given context.
func GetUserID(ctx context.Context) uuid.UUID {
	idStr := GetHeader(ctx, strings.ToLower(fhttp.HeaderXUserID))
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil
	}

	return id
}

// GetAccessToken returns the access token from the given context.
func GetAccessToken(ctx context.Context) string {
	s := GetHeader(ctx, strings.ToLower(fhttp.HeaderAuthorization))
	parts := strings.Split(s, " ")

	return parts[len(parts)-1]
}

// GetAuthenticated returns the authenticated flag from the given context.
func GetAuthenticated(ctx context.Context) bool {
	return GetHeader(ctx, strings.ToLower(fhttp.HeaderXAuthenticated)) == "true"
}
