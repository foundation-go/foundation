package errors

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/foundation-go/foundation/errors/proto"
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

// Ranging over the violations map directly produced a different response on
// every call for the same validation failure.
func TestInvalidArgumentErrorViolationsAreOrdered(t *testing.T) {
	violations := ErrorViolations{
		"name":  {ErrorCodeBlank},
		"email": {ErrorCodeInvalid, ErrorCodeTaken},
		"age":   {ErrorCodeNotANumber},
	}

	err := NewInvalidArgumentError("user", "1", violations)

	var fields []string
	for _, v := range err.MarshalProto().(*pb.InvalidArgumentError).Violations {
		fields = append(fields, v.Field)
	}

	assert.Equal(t, []string{"age", "email", "email", "name"}, fields)

	// Stable across repeated calls, not merely sorted once.
	for i := 0; i < 20; i++ {
		var again []string
		for _, v := range err.MarshalProto().(*pb.InvalidArgumentError).Violations {
			again = append(again, v.Field)
		}

		require.Equal(t, fields, again)
	}
}

func TestInvalidArgumentErrorGRPCStatusIsOrdered(t *testing.T) {
	err := NewInvalidArgumentError("user", "1", ErrorViolations{
		"b": {ErrorCodeBlank},
		"a": {ErrorCodeRequired},
	})

	details := err.GRPCStatus().Details()
	require.Len(t, details, 1)

	badRequest, ok := details[0].(*errdetails.BadRequest)
	require.True(t, ok)

	require.Len(t, badRequest.FieldViolations, 2)
	assert.Equal(t, "user/1#a", badRequest.FieldViolations[0].Field)
	assert.Equal(t, "user/1#b", badRequest.FieldViolations[1].Field)
}
