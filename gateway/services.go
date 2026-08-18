package gateway

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// defaultServiceConfig makes the gateway spread calls across every address a
// service resolves to.
//
// gRPC's default picker is `pick_first`: it opens one connection to the first
// address and sends everything there. Behind a Kubernetes headless service that
// means one pod serves all the traffic while its replicas sit idle, and a
// rolling deploy moves the whole load from pod to pod.
const defaultServiceConfig = `{"loadBalancingConfig":[{"round_robin":{}}]}`

// Service represents a gRPC service that can be registered with the gateway.
type Service struct {
	// Name of the service, used for environment variable lookup in the form of `GRPC_<NAME>_ENDPOINT`
	Name string

	// Function to register the gRPC server endpoint
	Register func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error
}

type RegisterServicesOptions struct {
	// Mux options
	MuxOpts []runtime.ServeMuxOption

	// TLS directory
	TLSDir string

	// DialOpts are appended to the defaults, and can override them.
	DialOpts []grpc.DialOption
}

// Mux is the gateway's request multiplexer, together with the lifetime of the
// connections to the downstream services.
type Mux struct {
	http.Handler

	closeOnce sync.Once
	cancel    context.CancelFunc
}

// Close releases the connections to the downstream services.
//
// grpc-gateway's generated registration closes each connection when the context
// it was given is cancelled. RegisterServices used to pass context.Background(),
// which never is — so the connections lived until the process exited and a
// shutdown dropped them rather than draining them.
func (m *Mux) Close() error {
	m.closeOnce.Do(func() {
		if m.cancel != nil {
			m.cancel()
		}
	})

	return nil
}

// RegisterServices builds the gateway mux and dials the downstream services.
//
// The connections live as long as ctx: grpc-gateway's generated registration
// closes each one when the context it was given is cancelled. Pass the service
// context, or call Mux.Close.
func RegisterServices(ctx context.Context, services []*Service, opts *RegisterServicesOptions) (*Mux, error) {
	ctx, cancel := context.WithCancel(ctx)

	// N.B.: the error handler is applied FIRST, so a caller-supplied
	// WithErrorHandler in MuxOpts wins. Appending it last, as this used to,
	// silently overrode the caller — the opposite of what GatewayOptions.MuxOpts
	// documents.
	muxOpts := make([]runtime.ServeMuxOption, 0, len(opts.MuxOpts)+1)
	muxOpts = append(muxOpts, runtime.WithErrorHandler(ErrorHandler))
	muxOpts = append(muxOpts, opts.MuxOpts...)

	mux := runtime.NewServeMux(muxOpts...)

	// Define gRPC connection options
	connCreds := insecure.NewCredentials()
	if opts.TLSDir != "" {
		var err error

		connCreds, err = newTLSCredentials(opts.TLSDir)
		if err != nil {
			cancel()

			return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
		}
	}

	grpcOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(connCreds),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithDefaultServiceConfig(defaultServiceConfig),
	}
	grpcOpts = append(grpcOpts, opts.DialOpts...)

	// Register gRPC server endpoints
	for _, service := range services {
		errPrefix := fmt.Sprintf("failed to register service `%s`", service.Name)

		// Fetch gRPC server endpoint from environment variable
		endpointVarName := fmt.Sprintf("GRPC_%s_ENDPOINT", strings.ToUpper(service.Name))

		endpoint := os.Getenv(endpointVarName)
		if endpoint == "" {
			cancel()

			return nil, fmt.Errorf("%s: environment variable `%s` is not set", errPrefix, endpointVarName)
		}

		if err := service.Register(ctx, mux, endpoint, grpcOpts); err != nil {
			cancel()

			return nil, fmt.Errorf("%s: %w", errPrefix, err)
		}
	}

	return &Mux{Handler: mux, cancel: cancel}, nil
}

// newTLSCredentials initializes a new TransportCredentials instance based on the
// files located in the provided directory. It expects three specific files to be present:
//
//  1. client.crt: This is the client's public certificate file. It's used to
//     authenticate the client to the server.
//
//  2. client.key: This is the client's private key file. This should be kept secret
//     and secure, as it's used in the encryption/decryption process.
//
//  3. ca.crt: This is the certificate authority (CA) certificate. This file is used
//     to validate server certificates for mutual TLS authentication.
func newTLSCredentials(dir string) (credentials.TransportCredentials, error) {
	certFile := fmt.Sprintf("%s/client.crt", dir)
	keyFile := fmt.Sprintf("%s/client.key", dir)
	caFile := fmt.Sprintf("%s/ca.crt", dir)

	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	// #nosec G304 -- the path is assembled from an operator-supplied
	// certificate directory, which is configuration rather than user input.
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS12,
	}), nil
}
