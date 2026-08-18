package grpc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	fctx "github.com/foundation-go/foundation/context"
)

func ctxWithMetadata(pairs map[string][]string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.MD(pairs))
}

func TestGetMetadataValue(t *testing.T) {
	assert.Equal(t, "", GetMetadataValue(context.Background(), "x-user-id"))
	assert.Equal(t, "", GetMetadataValue(ctxWithMetadata(nil), "x-user-id"))
	assert.Equal(t, "one", GetMetadataValue(ctxWithMetadata(map[string][]string{"k": {"one"}}), "k"))
}

// Joining duplicated values with a comma produced a string that is not a valid
// value of anything. For `x-scope` it was worse than useless: the joined string
// splits on whitespace into scopes nobody granted, so a duplicate value could
// smuggle a scope past CheckAnyScopePresence.
func TestGetMetadataValueDoesNotJoinDuplicates(t *testing.T) {
	ctx := ctxWithMetadata(map[string][]string{"x-scope": {"", "x admin"}})

	assert.Equal(t, "", GetMetadataValue(ctx, "x-scope"))

	scopes := getScopes(ctx)
	assert.False(t, scopes.ContainsAny("admin"), "a duplicated value must not grant a scope")
}

func TestGetMetadataValues(t *testing.T) {
	ctx := ctxWithMetadata(map[string][]string{"k": {"a", "b"}})

	assert.Equal(t, []string{"a", "b"}, GetMetadataValues(ctx, "k"))
	assert.Nil(t, GetMetadataValues(context.Background(), "k"))
	assert.Nil(t, GetMetadataValues(ctx, "missing"))
}

func TestMetadataUnaryInterceptorPopulatesTheContext(t *testing.T) {
	userID := uuid.New()
	clientID := uuid.New()

	ctx := ctxWithMetadata(map[string][]string{
		"x-correlation-id": {"corr-1"},
		"x-request-id":     {"req-1"},
		"x-authenticated":  {"true"},
		"x-user-id":        {userID.String()},
		"x-client-id":      {clientID.String()},
		"x-scope":          {"read write"},
		"authorization":    {"Bearer token"},
	})

	var got context.Context
	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		got = ctx
		return nil, nil
	}

	_, err := MetadataUnaryInterceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "corr-1", fctx.GetCorrelationID(got))
	assert.Equal(t, "req-1", fctx.GetRequestID(got))
	assert.True(t, fctx.GetAuthenticated(got))
	assert.Equal(t, userID, fctx.GetUserID(got))
	assert.Equal(t, clientID, fctx.GetClientID(got))
	assert.Equal(t, "token", fctx.GetAccessToken(got))
	assert.True(t, fctx.GetScopes(got).ContainsAll("read", "write"))
}

// Malformed or missing identity must degrade to the zero value, never to a
// panic and never to somebody else's identity.
func TestMetadataUnaryInterceptorWithoutMetadata(t *testing.T) {
	var got context.Context
	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		got = ctx
		return nil, nil
	}

	_, err := MetadataUnaryInterceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	require.NoError(t, err)

	assert.Equal(t, "", fctx.GetCorrelationID(got))
	assert.False(t, fctx.GetAuthenticated(got))
	assert.Equal(t, uuid.Nil, fctx.GetUserID(got))
	assert.Equal(t, uuid.Nil, fctx.GetClientID(got))
}

func TestMetadataUnaryInterceptorWithAMalformedUserID(t *testing.T) {
	ctx := ctxWithMetadata(map[string][]string{"x-user-id": {"not-a-uuid"}})

	var got context.Context
	_, err := MetadataUnaryInterceptor(ctx, nil, &grpc.UnaryServerInfo{},
		func(ctx context.Context, _ interface{}) (interface{}, error) {
			got = ctx
			return nil, nil
		})
	require.NoError(t, err)

	assert.Equal(t, uuid.Nil, fctx.GetUserID(got))
}
