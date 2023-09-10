package grpc

import (
	"context"

	"google.golang.org/grpc"

	ferr "github.com/ri-nat/foundation/errors"
)

func FoundationErrorToStatusUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	h, err := handler(ctx, req)

	if err != nil {
		if fErr, ok := err.(ferr.FoundationError); ok {
			return h, fErr.GRPCStatus().Err()
		}
	}

	return h, err
}
