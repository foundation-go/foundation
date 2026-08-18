package gateway

import (
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	fhttp "github.com/foundation-go/foundation/http"
)

// grpcMetadataPrefix is the prefix grpc-gateway's default header matcher strips
// before forwarding a header as gRPC metadata.
const grpcMetadataPrefix = "grpc-metadata-"

// IncomingHeaderMatcher is the default incoming header matcher for the gateway.
//
// It matches all Foundation headers and uses the default matcher for all other
// headers, except that it refuses the `Grpc-Metadata-` alias of any
// identity-bearing header.
func IncomingHeaderMatcher(key string) (string, bool) {
	// grpc-gateway's default matcher accepts any `Grpc-Metadata-Foo` header and
	// forwards `Foo` as gRPC metadata. That turns `Grpc-Metadata-X-User-Id`
	// into the very `x-user-id` metadata key downstream services trust, while
	// slipping past middleware that deletes `X-User-Id`. Identity must only
	// ever come from the header the gateway itself set.
	if isAliasedAuthenticationHeader(key) {
		return "", false
	}

	for _, header := range fhttp.FoundationHeaders {
		if strings.EqualFold(header, key) {
			return key, true
		}
	}

	return runtime.DefaultHeaderMatcher(key)
}

// OutgoingHeaderMatcher is the default outgoing header matcher for the gateway.
//
// It matches all Foundation headers and uses the default matcher for all other
// headers.
func OutgoingHeaderMatcher(key string) (string, bool) {
	return IncomingHeaderMatcher(key)
}

// isAliasedAuthenticationHeader reports whether key is a `Grpc-Metadata-`
// prefixed spelling of an identity-bearing Foundation header.
func isAliasedAuthenticationHeader(key string) bool {
	if len(key) <= len(grpcMetadataPrefix) || !strings.EqualFold(key[:len(grpcMetadataPrefix)], grpcMetadataPrefix) {
		return false
	}

	suffix := key[len(grpcMetadataPrefix):]

	for _, header := range fhttp.AuthenticationHeaders {
		if strings.EqualFold(header, suffix) {
			return true
		}
	}

	return false
}
