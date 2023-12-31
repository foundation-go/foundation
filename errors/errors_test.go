package errors

import (
	"fmt"
	"strings"
	"testing"

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

	// Check that the error can be converted to a gRPC s error
	s, ok := status.FromError(err)
	if !ok {
		t.Error("Expected a gRPC s error, but got a different error type")
	} else if s.Code() != codes.Internal {
		t.Errorf("Expected error code %s, but got %s", codes.Internal, s.Code())
	} else if s.Message() != "internal error" {
		t.Errorf("Expected error message '%s', but got '%s'", "internal error", s.Message())
	}
}
