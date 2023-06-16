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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux(muxOpts...)

	// Define gRPC connection options
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	// Register gRPC server endpoints
	for _, service := range services {
		// Fetch gRPC server endpoint from environment variable
		endpointVarName := fmt.Sprintf("GRPC_%s_ENDPOINT", strings.ToUpper(service.Name))
		endpoint := os.Getenv(endpointVarName)

		if err := service.Register(ctx, mux, endpoint, opts); err != nil {
			return nil, fmt.Errorf("failed to register gRPC endpoint for service `%s`: %v", service.Name, err)
		}
	}

	return mux, nil
}
