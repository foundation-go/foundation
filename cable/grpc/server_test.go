package cable_grpc

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/foundation-go/foundation/cable/grpc/proto"
)

func testServer(withAuth bool, authFn AuthenticationFunc) *Server {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	return &Server{
		Channels:           map[string]Channel{},
		WithAuthentication: withAuth,
		AuthenticationFunc: authFn,
		Logger:             logrus.NewEntry(logger),
	}
}

// A token in a URL is a token in every access log along the way. A header is
// preferred, with the query parameter kept for browser WebSocket clients, which
// cannot set headers.
func TestAccessTokenFrom(t *testing.T) {
	tests := []struct {
		name string
		env  *pb.Env
		want string
	}{
		{
			name: "authorization header",
			env:  &pb.Env{Headers: map[string]string{"authorization": "Bearer header-token"}},
			want: "header-token",
		},
		{
			name: "authorization header is matched case-insensitively",
			env:  &pb.Env{Headers: map[string]string{"Authorization": "Bearer header-token"}},
			want: "header-token",
		},
		{
			name: "bare token without the Bearer prefix",
			env:  &pb.Env{Headers: map[string]string{"authorization": "header-token"}},
			want: "header-token",
		},
		{
			name: "dedicated header",
			env:  &pb.Env{Headers: map[string]string{"X-Access-Token": "header-token"}},
			want: "header-token",
		},
		{
			name: "query parameter as a fallback",
			env:  &pb.Env{Url: "wss://example.com/cable?accessToken=query-token"},
			want: "query-token",
		},
		{
			name: "the header wins over the query parameter",
			env: &pb.Env{
				Url:     "wss://example.com/cable?accessToken=query-token",
				Headers: map[string]string{"authorization": "Bearer header-token"},
			},
			want: "header-token",
		},
		{name: "nothing at all", env: &pb.Env{Url: "wss://example.com/cable"}, want: ""},
		{name: "unparsable url", env: &pb.Env{Url: "://nonsense"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, accessTokenFrom(tt.env))
		})
	}
}

// The whole URL used to be written to the debug log, access token included.
func TestRedactAccessToken(t *testing.T) {
	redacted := redactAccessToken("wss://example.com/cable?accessToken=secret&foo=bar")

	assert.NotContains(t, redacted, "secret")
	assert.Contains(t, redacted, "REDACTED")
	assert.Contains(t, redacted, "foo=bar")

	assert.Equal(t, "wss://example.com/cable", redactAccessToken("wss://example.com/cable"))
	assert.Equal(t, "<unparsable url>", redactAccessToken("://nonsense"))
}

// WithAuthentication with no AuthenticationFunc dereferenced nil and took the
// server down. It has to fail closed instead.
func TestConnectWithoutAnAuthenticationFunc(t *testing.T) {
	server := testServer(true, nil)

	var (
		resp *pb.ConnectionResponse
		err  error
	)

	require.NotPanics(t, func() {
		resp, err = server.Connect(context.Background(), &pb.ConnectionRequest{
			Env: &pb.Env{Url: "wss://example.com/cable?accessToken=secret"},
		})
	})

	require.NoError(t, err)
	assert.Equal(t, pb.Status_FAILURE, resp.Status)
}

func TestConnectAuthenticates(t *testing.T) {
	server := testServer(true, func(_ context.Context, token string) (string, error) {
		if token != "good" {
			return "", errors.New("invalid token")
		}

		return "user-1", nil
	})

	t.Run("valid token", func(t *testing.T) {
		resp, err := server.Connect(context.Background(), &pb.ConnectionRequest{
			Env: &pb.Env{Headers: map[string]string{"authorization": "Bearer good"}},
		})
		require.NoError(t, err)

		assert.Equal(t, pb.Status_SUCCESS, resp.Status)
		assert.Equal(t, "user-1", resp.Env.Cstate[UserIDKey])
		assert.Equal(t, "true", resp.Env.Cstate[IsAuthenticatedKey])
	})

	t.Run("invalid token", func(t *testing.T) {
		resp, err := server.Connect(context.Background(), &pb.ConnectionRequest{
			Env: &pb.Env{Headers: map[string]string{"authorization": "Bearer bad"}},
		})
		require.NoError(t, err)

		assert.Equal(t, pb.Status_FAILURE, resp.Status)
	})

	t.Run("no token", func(t *testing.T) {
		resp, err := server.Connect(context.Background(), &pb.ConnectionRequest{Env: &pb.Env{}})
		require.NoError(t, err)

		assert.Equal(t, pb.Status_FAILURE, resp.Status)
	})
}

func TestConnectWithoutAuthentication(t *testing.T) {
	resp, err := testServer(false, nil).Connect(context.Background(), &pb.ConnectionRequest{Env: &pb.Env{}})
	require.NoError(t, err)

	assert.Equal(t, pb.Status_SUCCESS, resp.Status)
	assert.Equal(t, "false", resp.Env.Cstate[IsAuthenticatedKey])
}

// A request with no env used to dereference nil.
func TestRequestsWithoutAnEnv(t *testing.T) {
	server := testServer(false, nil)

	require.NotPanics(t, func() {
		_, err := server.Connect(context.Background(), &pb.ConnectionRequest{})
		assert.Error(t, err)
	})

	require.NotPanics(t, func() {
		_, err := server.Command(context.Background(), &pb.CommandMessage{})
		assert.Error(t, err)
	})
}
