package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ferr "github.com/foundation-go/foundation/errors"
)

func TestFoundationErrorToStatusInterceptor(t *testing.T) {
	// Define a mock handler that returns an error
	mockHandler := func(context.Context, interface{}) (interface{}, error) {
		return nil, ferr.NewNotFoundError(nil, "test", "123")
	}

	ctx := context.Background()

	// Call the interceptor with the mock handler and check that it returns the expected error status
	_, err := FoundationErrorToStatusUnaryInterceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)
	if err == nil {
		t.Error("Expected an error, but got nil")
	} else {
		s, ok := status.FromError(err)
		if !ok {
			t.Error("Expected a gRPC s error, but got a different error type")
		} else if s.Code() != codes.NotFound {
			t.Errorf("Expected error code %s, but got %s", codes.NotFound, s.Code())
		} else if s.Message() != "not found: test/123" {
			t.Errorf("Expected error message 'not found: test/123', but got '%s'", s.Message())
		}
	}

	// Call the interceptor with a mock handler that returns no error and check that it returns no error
	mockHandler = func(ctx context.Context, req interface{}) (interface{}, error) {
		return "test", nil
	}

	_, err = FoundationErrorToStatusUnaryInterceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}

	// Call the interceptor with a mock handler that returns a status error and check that it returns the expected error status
	mockHandler = func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, status.Error(codes.NotFound, "not found")
	}

	_, err = FoundationErrorToStatusUnaryInterceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)
	if err == nil {
		t.Error("Expected an error, but got nil")
	} else {
		s, ok := status.FromError(err)
		if !ok {
			t.Error("Expected a gRPC s error, but got a different error type")
		} else if s.Code() != codes.NotFound {
			t.Errorf("Expected error code %s, but got %s", codes.NotFound, s.Code())
		} else if s.Message() != "not found" {
			t.Errorf("Expected error message 'not found', but got '%s'", s.Message())
		}
	}
}
