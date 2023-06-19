package grpc

import (
	"fmt"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewInternalError creates a generic internal error.
func NewInternalError(err error, msg string) error {
	return status.Errorf(codes.Internal, "internal error")
}

// NewInvalidArgumentError creates an invalid argument error with error details.
func NewInvalidArgumentError(kind string, id string, violations map[string][]string) error {
	obj := fmt.Sprintf("%s/%s", kind, id)
	msg := "validation error"

	// Create status with error message
	st := status.New(codes.InvalidArgument, msg)

	// Attach error details
	badRequest := &errdetails.BadRequest{}
	for field, description := range violations {
		for _, d := range description {
			badRequest.FieldViolations = append(badRequest.FieldViolations, &errdetails.BadRequest_FieldViolation{
				Field:       fmt.Sprintf("%s#%s", obj, field),
				Description: d,
			})
		}
	}

	st, err := st.WithDetails(badRequest)
	if err != nil {
		return status.Error(codes.Internal, "internal error")
	}

	return st.Err()
}

// NewNotFoundError creates a not found error.
func NewNotFoundError(err error, msg string) error {
	return status.Errorf(codes.NotFound, msg)
}

// NewPermissionDeniedError creates a permission denied error.
func NewPermissionDeniedError(err error, msg string) error {
	return status.Errorf(codes.PermissionDenied, msg)
}

// NewStaleObjectError creates a stale object error.
func NewStaleObjectError(kind string, id string, actualVersion, expectedVersion int32) error {
	msg := fmt.Sprintf("stale object: %s/%s", kind, id)

	// Create status with error message
	st := status.New(codes.FailedPrecondition, msg)

	// Attach error details
	st, err := st.WithDetails(&errdetails.PreconditionFailure{
		Violations: []*errdetails.PreconditionFailure_Violation{{
			Type:        "stale_object",
			Subject:     fmt.Sprintf("%s/%s", kind, id),
			Description: fmt.Sprintf("actual version: %d, expected version: %d", actualVersion, expectedVersion),
		}},
	})
	if err != nil {
		return status.Error(codes.Internal, "internal error")
	}

	return st.Err()
}

// NewUnauthenticatedError creates an unauthenticated error.
func NewUnauthenticatedError(err error, msg string) error {
	return status.Errorf(codes.Unauthenticated, msg)
}
