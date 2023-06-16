package gateway

import (
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	fhttp "github.com/ri-nat/foundation/http"
)

func IncomingHeaderMatcher(key string) (string, bool) {
	switch key {
	case fhttp.HeaderXCorrelationID, fhttp.HeaderXAuthenticated, fhttp.HeaderXUserID:
		return key, true
	default:
		return runtime.DefaultHeaderMatcher(key)
	}
}
