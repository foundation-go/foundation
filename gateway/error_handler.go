package gateway

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	fctx "github.com/ri-nat/foundation/context"
)

func ErrorHandler(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
	log := fctx.GetLogger(r.Context())

	switch code := status.Code(err); code {
	case codes.Internal:
		log.Errorf("internal error: %v", err)

		// TODO: log error to Sentry

		err = &runtime.HTTPStatusError{
			HTTPStatus: http.StatusInternalServerError,
			Err:        status.Error(code, http.StatusText(http.StatusInternalServerError)),
		}
	}

	runtime.DefaultHTTPErrorHandler(ctx, mux, marshaler, w, r, err)
}
