package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestRegisterServicesRequiresAnEndpoint(t *testing.T) {
	_, err := RegisterServices(
		context.Background(),
		[]*Service{{Name: "chats", Register: func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error {
			return nil
		}}},
		&RegisterServicesOptions{},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GRPC_CHATS_ENDPOINT")
}

// gRPC's default picker is pick_first, which sends everything to a single
// address: behind a headless Kubernetes service one pod takes all the traffic
// while its replicas sit idle.
func TestRegisterServicesEnablesRoundRobin(t *testing.T) {
	t.Setenv("GRPC_CHATS_ENDPOINT", "chats:51051")

	var gotOpts []grpc.DialOption

	mux, err := RegisterServices(
		context.Background(),
		[]*Service{{
			Name: "chats",
			Register: func(_ context.Context, _ *runtime.ServeMux, _ string, opts []grpc.DialOption) error {
				gotOpts = opts
				return nil
			},
		}},
		&RegisterServicesOptions{},
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mux.Close() })

	// The service config is opaque once wrapped in a DialOption, so assert on
	// the constant the option is built from and that some options were passed.
	assert.Contains(t, defaultServiceConfig, "round_robin")
	assert.NotEmpty(t, gotOpts)
}

// The registration context is what the generated code hangs each connection's
// lifetime off; it used to be context.Background(), which is never cancelled.
func TestMuxCloseCancelsTheRegistrationContext(t *testing.T) {
	t.Setenv("GRPC_CHATS_ENDPOINT", "chats:51051")

	var registrationCtx context.Context

	mux, err := RegisterServices(
		context.Background(),
		[]*Service{{
			Name: "chats",
			Register: func(ctx context.Context, _ *runtime.ServeMux, _ string, _ []grpc.DialOption) error {
				registrationCtx = ctx
				return nil
			},
		}},
		&RegisterServicesOptions{},
	)
	require.NoError(t, err)
	require.NotNil(t, registrationCtx)

	assert.NoError(t, registrationCtx.Err(), "the context must stay live while serving")

	require.NoError(t, mux.Close())
	assert.Error(t, registrationCtx.Err(), "closing the mux must release the connections")

	// Closing twice is safe.
	assert.NoError(t, mux.Close())
}

// GatewayOptions.MuxOpts documents that caller options override Foundation's
// defaults, but WithErrorHandler was appended last and silently won.
func TestCallerErrorHandlerOverridesTheDefault(t *testing.T) {
	t.Setenv("GRPC_CHATS_ENDPOINT", "chats:51051")

	called := false

	mux, err := RegisterServices(
		context.Background(),
		[]*Service{{
			Name:     "chats",
			Register: func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error { return nil },
		}},
		&RegisterServicesOptions{
			MuxOpts: []runtime.ServeMuxOption{
				runtime.WithErrorHandler(func(
					_ context.Context, _ *runtime.ServeMux, _ runtime.Marshaler,
					w http.ResponseWriter, _ *http.Request, _ error,
				) {
					called = true
					w.WriteHeader(http.StatusTeapot)
				}),
			},
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mux.Close() })

	// An unrouted request goes through the error handler.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nowhere", nil))

	assert.True(t, called, "the caller's error handler should have been used")
	assert.Equal(t, http.StatusTeapot, rec.Code)
}
