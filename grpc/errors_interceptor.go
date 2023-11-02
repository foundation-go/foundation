package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc"

	ferr "github.com/foundation-go/foundation/errors"
)

func FoundationErrorToStatusUnaryInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	h, err := handler(ctx, req)

	if err != nil {
		var fErr ferr.FoundationError
		if errors.As(err, &fErr) {
			return h, fErr.GRPCStatus().Err()
		}
	}

	return h, err
}
