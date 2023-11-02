package errors

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/pkg/errors"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pb "github.com/foundation-go/foundation/errors/proto"
)

// BaseError is the base error type for all errors in the Foundation framework.
type BaseError struct {
	Err error
}

func (e *BaseError) Error() string {
	return e.Err.Error()
}

// GRPCStatus returns a gRPC status error for the Foundation error.
func (e *BaseError) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Err.Error())
}

// ErrorCode describes an error code.
type ErrorCode string

// Generic error codes.
const (
	ErrorCodeAccepted     ErrorCode = "Accepted"     // ErrorCodeAccepted - Must be accepted
	ErrorCodeBlank        ErrorCode = "Blank"        // ErrorCodeBlank - Can't be blank
	ErrorCodeEmpty        ErrorCode = "Empty"        // ErrorCodeEmpty - Can't be empty
	ErrorCodeEven         ErrorCode = "Even"         // ErrorCodeEven - Must be even
	ErrorCodeExclusion    ErrorCode = "Exclusion"    // ErrorCodeExclusion - Is reserved
	ErrorCodeInclusion    ErrorCode = "Inclusion"    // ErrorCodeInclusion - Is not included in the list
	ErrorCodeInvalid      ErrorCode = "Invalid"      // ErrorCodeInvalid - Is invalid
	ErrorCodeNotANumber   ErrorCode = "NotANumber"   // ErrorCodeNotANumber - Is not a number
	ErrorCodeNotAnInteger ErrorCode = "NotAnInteger" // ErrorCodeNotAnInteger - Must be an integer
	ErrorCodeOdd          ErrorCode = "Odd"          // ErrorCodeOdd - Must be odd
	ErrorCodePresent      ErrorCode = "Present"      // ErrorCodePresent - Must be blank
	ErrorCodeRequired     ErrorCode = "Required"     // ErrorCodeRequired - Must exist
	ErrorCodeTaken        ErrorCode = "Taken"        // ErrorCodeTaken - Has already been taken
)

// String implements the Stringer interface.
func (i ErrorCode) String() string {
	return string(i)
}

// FoundationError describes an interface for all errors in the Foundation framework.
type FoundationError interface {
	error
	GRPCStatus() *status.Status
	MarshalProto() proto.Message
	MarshalJSON() ([]byte, error)
}

// InternalError describes an internal error
type InternalError struct {
	*BaseError
}

func (e *InternalError) GRPCStatus() *status.Status {
	return status.New(codes.Internal, "internal error")
}

// MarshalProto marshals the error to a proto.Message.
func (e *InternalError) MarshalProto() proto.Message {
	return &pb.InternalError{}
}

func (e *InternalError) MarshalJSON() ([]byte, error) {
	return []byte("{}"), nil
}

// NewInternalError creates a generic internal error.
func NewInternalError(err error, msg string) *InternalError {
	return &InternalError{
		BaseError: &BaseError{
			Err: errors.Wrap(err, msg),
		},
	}
}

// ErrorViolations describes a map of field names to error codes.
type ErrorViolations = map[string][]fmt.Stringer

// InvalidArgumentError describes an invalid argument error.
type InvalidArgumentError struct {
	*BaseError

	Kind       string
	ID         string
	Violations ErrorViolations
}

func (e *InvalidArgumentError) GRPCStatus() *status.Status {
	obj := fmt.Sprintf("%s/%s", e.Kind, e.ID)
	msg := "validation error"

	// Create status with error message
	st := status.New(codes.InvalidArgument, msg)

	// Attach error details
	badRequest := &errdetails.BadRequest{}
	for field, description := range e.Violations {
		for _, d := range description {
			badRequest.FieldViolations = append(badRequest.FieldViolations, &errdetails.BadRequest_FieldViolation{
				Field:       fmt.Sprintf("%s#%s", obj, field),
				Description: d.String(),
			})
		}
	}

	st, err := st.WithDetails(badRequest)
	if err != nil {
		sentry.CaptureException(err)
		return status.New(codes.Internal, "internal error")
	}

	return st
}

// MarshalProto marshals the error to a proto.Message.
func (e *InvalidArgumentError) MarshalProto() proto.Message {
	err := &pb.InvalidArgumentError{
		Kind: e.Kind,
		Id:   e.ID,
	}

	for field, description := range e.Violations {
		for _, d := range description {
			err.Violations = append(err.Violations, &pb.InvalidArgumentError_Violation{
				Field:       field,
				Description: d.String(),
			})
		}
	}

	return err
}

// MarshalJSON marshals the error to JSON.
func (e *InvalidArgumentError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.MarshalProto())
}

// NewInvalidArgumentError creates an invalid argument error with error details.
func NewInvalidArgumentError(kind string, id string, violations map[string][]fmt.Stringer) *InvalidArgumentError {
	return &InvalidArgumentError{
		BaseError: &BaseError{
			Err: fmt.Errorf("invalid argument: %s/%s", kind, id),
		},
		Kind:       kind,
		ID:         id,
		Violations: violations,
	}
}

// NotFoundError describes a not found error.
type NotFoundError struct {
	*BaseError

	Kind string
	ID   string
}

func (e *NotFoundError) GRPCStatus() *status.Status {
	msg := fmt.Sprintf("not found: %s/%s", e.Kind, e.ID)

	return status.New(codes.NotFound, msg)
}

// MarshalProto marshals the error to a proto.Message.
func (e *NotFoundError) MarshalProto() proto.Message {
	return &pb.NotFoundError{
		Kind: e.Kind,
		Id:   e.ID,
	}
}

// MarshalJSON marshals the error to JSON.
func (e *NotFoundError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.MarshalProto())
}

// NewNotFoundError creates a not found error.
func NewNotFoundError(err error, kind string, id string) *NotFoundError {
	return &NotFoundError{
		BaseError: &BaseError{
			Err: err,
		},
		Kind: kind,
		ID:   id,
	}
}

// PermissionDeniedError describes a permission denied error.
type PermissionDeniedError struct {
	*BaseError

	Action string
	Kind   string
	ID     string
}

func (e *PermissionDeniedError) GRPCStatus() *status.Status {
	return status.New(codes.PermissionDenied, e.Err.Error())
}

// MarshalProto marshals the error to a proto.Message.
func (e *PermissionDeniedError) MarshalProto() proto.Message {
	return &pb.PermissionDeniedError{
		Action: e.Action,
		Kind:   e.Kind,
		Id:     e.ID,
	}
}

// MarshalJSON marshals the error to JSON.
func (e *PermissionDeniedError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.MarshalProto())
}

// NewPermissionDeniedError creates a permission denied error.
func NewPermissionDeniedError(action string, kind string, id string) *PermissionDeniedError {
	err := fmt.Errorf("permission denied: `%s` on %s/%s", action, kind, id)

	return &PermissionDeniedError{
		BaseError: &BaseError{
			Err: err,
		},
		Action: action,
		Kind:   kind,
		ID:     id,
	}
}

// NewInsufficientScopeAllError creates an error for cases where all the expected
// scopes are required.
func NewInsufficientScopeAllError(expectedScopes ...string) *PermissionDeniedError {
	return &PermissionDeniedError{
		BaseError: &BaseError{
			Err: fmt.Errorf("insufficient scope: expected to be `%v`", strings.Join(expectedScopes, " ")),
		},
	}
}

// NewInsufficientScopeAnyError creates an error for cases where
// any of the expected scopes is required.
func NewInsufficientScopeAnyError(expectedScopes ...string) *PermissionDeniedError {
	return &PermissionDeniedError{
		BaseError: &BaseError{
			Err: fmt.Errorf("insufficient scope: expected to be any of `%v`", strings.Join(expectedScopes, " ")),
		},
	}
}

// UnauthenticatedError describes an unauthenticated error.
type UnauthenticatedError struct {
	*BaseError
}

func (e *UnauthenticatedError) GRPCStatus() *status.Status {
	return status.New(codes.Unauthenticated, e.Err.Error())
}

// MarshalProto marshals the error to a proto.Message.
func (e *UnauthenticatedError) MarshalProto() proto.Message {
	return &pb.UnauthenticatedError{}
}

// MarshalJSON marshals the error to JSON.
func (e *UnauthenticatedError) MarshalJSON() ([]byte, error) {
	return []byte("{}"), nil
}

// NewUnauthenticatedError creates an unauthenticated error.
func NewUnauthenticatedError(msg string) *UnauthenticatedError {
	return &UnauthenticatedError{
		BaseError: &BaseError{
			Err: fmt.Errorf("unauthenticated: %s", msg),
		},
	}
}

// StaleObjectError describes a stale object error.
type StaleObjectError struct {
	*BaseError

	Kind            string
	ID              string
	ActualVersion   int32
	ExpectedVersion int32
}

func (e *StaleObjectError) GRPCStatus() *status.Status {
	msg := fmt.Sprintf("stale object: %s/%s", e.Kind, e.ID)

	// Create status with error message
	st := status.New(codes.FailedPrecondition, msg)

	// Attach error details
	st, err := st.WithDetails(&errdetails.PreconditionFailure{
		Violations: []*errdetails.PreconditionFailure_Violation{{
			Type:        "stale_object",
			Subject:     fmt.Sprintf("%s/%s", e.Kind, e.ID),
			Description: fmt.Sprintf("actual version: %d, expected version: %d", e.ActualVersion, e.ExpectedVersion),
		}},
	})
	if err != nil {
		// TODO: maybe `fatal` here?
		return status.New(codes.Internal, "internal error")
	}

	return st
}

// MarshalProto marshals the error to a proto.Message.
func (e *StaleObjectError) MarshalProto() proto.Message {
	return &pb.StaleObjectError{
		Kind:            e.Kind,
		Id:              e.ID,
		ActualVersion:   e.ActualVersion,
		ExpectedVersion: e.ExpectedVersion,
	}
}

// MarshalJSON marshals the error to JSON.
func (e *StaleObjectError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.MarshalProto())
}

// NewStaleObjectError creates a stale object error.
func NewStaleObjectError(kind string, id string, actualVersion, expectedVersion int32) *StaleObjectError {
	return &StaleObjectError{
		BaseError: &BaseError{
			Err: fmt.Errorf("stale object: %s/%s", kind, id),
		},
		Kind:            kind,
		ID:              id,
		ActualVersion:   actualVersion,
		ExpectedVersion: expectedVersion,
	}
}
