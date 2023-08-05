package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/grpc/credentials"
)

// NewTLSConfig initializes a new TransportCredentials instance based on the files
// located in the provided tlsDir directory. It expects three specific files to be present:
//
//  1. server.crt: This is the server's public certificate file. It's used to
//     authenticate the server to the client.
//
//  2. server.key: This is the server's private key file. This should be kept secret
//     and secure, as it's used in the encryption/decryption process.
//
//  3. ca.crt: This is the certificate authority (CA) certificate. This file is used
//     to validate client certificates for mutual TLS authentication.
func NewTLSConfig(tlsDir string) (credentials.TransportCredentials, error) {
	certFile := filepath.Join(tlsDir, "server.crt")
	keyFile := filepath.Join(tlsDir, "server.key")
	caFile := filepath.Join(tlsDir, "ca.crt")

	peerCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{peerCert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}), nil
}
