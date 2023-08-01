package gateway

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Service represents a gRPC service that can be registered with the gateway.
type Service struct {
	// Name of the service, used for environment variable lookup in the form of `GRPC_<NAME>_ENDPOINT`
	Name string

	// Function to register the gRPC server endpoint
	Register func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error
}

func RegisterServices(services []Service, muxOpts ...runtime.ServeMuxOption) (http.Handler, error) {
	ctx := context.Background()
	mux := runtime.NewServeMux(muxOpts...)

	// Define gRPC connection options
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	// Register gRPC server endpoints
	for _, service := range services {
		errPrefix := fmt.Sprintf("failed to register service `%s`", service.Name)

		// Fetch gRPC server endpoint from environment variable
		endpointVarName := fmt.Sprintf("GRPC_%s_ENDPOINT", strings.ToUpper(service.Name))
		endpoint := os.Getenv(endpointVarName)
		if endpoint == "" {
			return nil, fmt.Errorf("%s: environment variable `%s` is not set", errPrefix, endpointVarName)
		}

		if err := service.Register(ctx, mux, endpoint, opts); err != nil {
			return nil, fmt.Errorf("%s: %v", errPrefix, err)
		}
	}

	return mux, nil
}
