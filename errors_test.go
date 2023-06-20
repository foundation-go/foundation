package foundation

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestInternalError(t *testing.T) {
	// Create a new internal error
	err := NewInternalError(fmt.Errorf("test error"), "test")

	// Check that the error message and details are correct
	expectedSuff := "test: test error"
	if !strings.HasSuffix(err.Error(), expectedSuff) {
		t.Errorf("Expected error message to end with '%s', but got '%s'", expectedSuff, err.Error())
	}

	// Check that the error can be converted to a gRPC status error
	status, ok := status.FromError(err)
	if !ok {
		t.Error("Expected a gRPC status error, but got a different error type")
	} else if status.Code() != codes.Internal {
		t.Errorf("Expected error code %s, but got %s", codes.Internal, status.Code())
	} else if status.Message() != "internal error" {
		t.Errorf("Expected error message '%s', but got '%s'", "internal error", status.Message())
	}
}

func TestFoundationErrorToStatusInterceptor(t *testing.T) {
	app := Init("test-app")

	// Define a mock handler that returns an error
	mockHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, NewNotFoundError(nil, "test", "123")
	}

	// Call the interceptor with the mock handler and check that it returns the expected error status
	_, err := app.foundationErrorToStatusInterceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, mockHandler)
	if err == nil {
		t.Error("Expected an error, but got nil")
	} else {
		status, ok := status.FromError(err)
		if !ok {
			t.Error("Expected a gRPC status error, but got a different error type")
		} else if status.Code() != codes.NotFound {
			t.Errorf("Expected error code %s, but got %s", codes.NotFound, status.Code())
		} else if status.Message() != "not found: test/123" {
			t.Errorf("Expected error message 'not found: test/123', but got '%s'", status.Message())
		}
	}

	// Call the interceptor with a mock handler that returns no error and check that it returns no error
	mockHandler = func(ctx context.Context, req interface{}) (interface{}, error) {
		return "test", nil
	}

	_, err = app.foundationErrorToStatusInterceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, mockHandler)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}

	// Call the interceptor with a mock handler that returns a status error and check that it returns the expected error status
	mockHandler = func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, status.Error(codes.NotFound, "not found")
	}

	_, err = app.foundationErrorToStatusInterceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, mockHandler)
	if err == nil {
		t.Error("Expected an error, but got nil")
	} else {
		status, ok := status.FromError(err)
		if !ok {
			t.Error("Expected a gRPC status error, but got a different error type")
		} else if status.Code() != codes.NotFound {
			t.Errorf("Expected error code %s, but got %s", codes.NotFound, status.Code())
		} else if status.Message() != "not found" {
			t.Errorf("Expected error message 'not found', but got '%s'", status.Message())
		}
	}
}
