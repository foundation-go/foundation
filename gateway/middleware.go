package gateway

import (
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	fhttp "github.com/ri-nat/foundation/http"
)

func IncomingHeaderMatcher(key string) (string, bool) {
	for _, header := range fhttp.FoundationHeaders {
		if header == key {
			return key, true
		}
	}

	return runtime.DefaultHeaderMatcher(key)
}
