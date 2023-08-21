package gateway

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

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
}

func RegisterServices(services []*Service, opts *RegisterServicesOptions) (http.Handler, error) {
	ctx := context.Background()

	muxOpts := append(opts.MuxOpts, runtime.WithErrorHandler(ErrorHandler))
	mux := runtime.NewServeMux(muxOpts...)

	// Define gRPC connection options
	connCreds := insecure.NewCredentials()
	if opts.TLSDir != "" {
		var err error
		connCreds, err = newTLSCredentials(opts.TLSDir)

		if err != nil {
			return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
		}
	}

	grpcOpts := []grpc.DialOption{grpc.WithTransportCredentials(connCreds)}

	// Register gRPC server endpoints
	for _, service := range services {
		errPrefix := fmt.Sprintf("failed to register service `%s`", service.Name)

		// Fetch gRPC server endpoint from environment variable
		endpointVarName := fmt.Sprintf("GRPC_%s_ENDPOINT", strings.ToUpper(service.Name))
		endpoint := os.Getenv(endpointVarName)
		if endpoint == "" {
			return nil, fmt.Errorf("%s: environment variable `%s` is not set", errPrefix, endpointVarName)
		}

		if err := service.Register(ctx, mux, endpoint, grpcOpts); err != nil {
			return nil, fmt.Errorf("%s: %v", errPrefix, err)
		}
	}

	return mux, nil
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
	}), nil
}
