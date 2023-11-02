package gateway

import (
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	fhttp "github.com/foundation-go/foundation/http"
)

// IncomingHeaderMatcher is the default incoming header matcher for the gateway.
//
// It matches all Foundation headers and uses the default matcher for all other headers.
func IncomingHeaderMatcher(key string) (string, bool) {
	for _, header := range fhttp.FoundationHeaders {
		if strings.EqualFold(header, key) {
			return key, true
		}
	}

	return runtime.DefaultHeaderMatcher(key)
}

// OutgoingHeaderMatcher is the default outgoing header matcher for the gateway.
//
// It matches all Foundation headers and uses the default matcher for all other headers.
func OutgoingHeaderMatcher(key string) (string, bool) {
	return IncomingHeaderMatcher(key)
}
