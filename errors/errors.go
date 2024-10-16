package errors

import (
	"encoding/json"
	"fmt"
	"github.com/getsentry/sentry-go"
	"strings"

	"github.com/pkg/errors"
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
	Code    uint32
	Message string
}

func (e *InternalError) GRPCStatus() *status.Status {
	detailErr := &pb.Error{
		Code:    e.Code,
		Message: e.Message,
	}

	st := status.New(codes.Internal, "internal error")
	st, err := st.WithDetails(detailErr)
	if err != nil {
		sentry.CaptureException(err)
	}

	return st
}

// MarshalProto marshals the error to a proto.Message.
func (e *InternalError) MarshalProto() proto.Message {
	return &pb.Error{
		Code:    e.Code,
		Message: e.Message,
	}
}

func (e *InternalError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.MarshalProto())
}

// NewInternalError creates a generic internal error.
func NewInternalError(err error, message string) *InternalError {
	return &InternalError{
		BaseError: &BaseError{
			Err: errors.Wrap(err, message),
		},
		Code:    uint32(codes.Internal),
		Message: message,
	}
}

// ErrorViolations describes a map of field names to error codes.
type ErrorViolations = map[string][]fmt.Stringer

// InvalidArgumentError describes an invalid argument error.
type InvalidArgumentError struct {
	*BaseError

	Code       uint32
	Violations ErrorViolations
}

func (e *InvalidArgumentError) GRPCStatus() *status.Status {
	detailErr := &pb.Error{
		Code:    e.Code,
		Message: e.Error(),
		Details: make(map[string]string, len(e.Violations)),
	}

	for field, description := range e.Violations {
		for _, d := range description {
			detailErr.Details[field] = d.String()
		}
	}

	st := status.New(codes.InvalidArgument, e.Error())
	st, err := st.WithDetails(detailErr)
	if err != nil {
		sentry.CaptureException(err)
	}

	return st
}

// MarshalProto marshals the error to a proto.Message.
func (e *InvalidArgumentError) MarshalProto() proto.Message {
	err := &pb.Error{
		Code:    e.Code,
		Message: e.Error(),
		Details: make(map[string]string, len(e.Violations)),
	}

	for field, description := range e.Violations {
		for _, d := range description {
			err.Details[field] = d.String()
		}
	}

	return err
}

// MarshalJSON marshals the error to JSON.
func (e *InvalidArgumentError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.MarshalProto())
}

// NewInvalidArgumentError creates an invalid argument error with error details.
func NewInvalidArgumentError(kind string, id string, violations map[string][]fmt.Stringer, code uint32) *InvalidArgumentError {
	return &InvalidArgumentError{
		BaseError: &BaseError{
			Err: fmt.Errorf("invalid argument: %s/%s", kind, id),
		},
		Violations: violations,
		Code:       code,
	}
}

// NotFoundError describes a not found error.
type NotFoundError struct {
	*BaseError

	Kind    string
	ID      string
	Code    uint32
	Message string
}

func (e *NotFoundError) GRPCStatus() *status.Status {
	detailErr := &pb.Error{
		Code:    e.Code,
		Message: e.Message,
	}

	st := status.New(codes.NotFound, e.Message)
	st, err := st.WithDetails(detailErr)
	if err != nil {
		sentry.CaptureException(err)
	}

	return st
}

// MarshalProto marshals the error to a proto.Message.
func (e *NotFoundError) MarshalProto() proto.Message {
	return &pb.Error{
		Message: e.Message,
		Code:    e.Code,
	}
}

// MarshalJSON marshals the error to JSON.
func (e *NotFoundError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.MarshalProto())
}

// NewNotFoundError creates a not found error.
func NewNotFoundError(err error, kind string, id string, code uint32) *NotFoundError {
	return &NotFoundError{
		BaseError: &BaseError{
			Err: err,
		},
		Kind:    kind,
		ID:      id,
		Code:    code,
		Message: fmt.Sprintf(`not found: %s/%s`, kind, id),
	}
}

// PermissionDeniedError describes a permission denied error.
type PermissionDeniedError struct {
	*BaseError

	Action string
	Code   uint32
}

func (e *PermissionDeniedError) GRPCStatus() *status.Status {
	detailErr := &pb.Error{
		Code:    e.Code,
		Message: e.Error(),
	}

	st := status.New(codes.PermissionDenied, e.Error())
	st, err := st.WithDetails(detailErr)
	if err != nil {
		sentry.CaptureException(err)
	}

	return st
}

// MarshalProto marshals the error to a proto.Message.
func (e *PermissionDeniedError) MarshalProto() proto.Message {
	return &pb.Error{
		Message: e.Error(),
		Code:    e.Code,
	}
}

// MarshalJSON marshals the error to JSON.
func (e *PermissionDeniedError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.MarshalProto())
}

// NewPermissionDeniedError creates a permission denied error.
func NewPermissionDeniedError(action string, kind string, id string, code uint32) *PermissionDeniedError {
	return &PermissionDeniedError{
		BaseError: &BaseError{
			Err: fmt.Errorf("permission denied: `%s` on %s/%s", action, kind, id),
		},
		Code: code,
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

	Code uint32
}

func (e *UnauthenticatedError) GRPCStatus() *status.Status {
	detailErr := &pb.Error{
		Code:    e.Code,
		Message: e.Error(),
	}

	st := status.New(codes.Unauthenticated, e.Error())
	st, err := st.WithDetails(detailErr)
	if err != nil {
		sentry.CaptureException(err)
	}

	return st
}

// MarshalProto marshals the error to a proto.Message.
func (e *UnauthenticatedError) MarshalProto() proto.Message {
	return &pb.Error{
		Code:    e.Code,
		Message: e.Error(),
	}
}

// MarshalJSON marshals the error to JSON.
func (e *UnauthenticatedError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.MarshalProto())
}

// NewUnauthenticatedError creates an unauthenticated error.
func NewUnauthenticatedError(msg string, code uint32) *UnauthenticatedError {
	return &UnauthenticatedError{
		BaseError: &BaseError{
			Err: fmt.Errorf("unauthenticated: %s", msg),
		},
		Code: code,
	}
}

// StaleObjectError describes a stale object error.
type StaleObjectError struct {
	*BaseError

	Kind            string
	ID              string
	ActualVersion   int32
	ExpectedVersion int32
	Code            uint32
}

func (e *StaleObjectError) GRPCStatus() *status.Status {
	detailErr := &pb.Error{
		Code:    e.Code,
		Message: e.Error(),
		Details: map[string]string{
			`kind`:             e.Kind,
			`id`:               e.ID,
			`actual_version`:   string(e.ActualVersion),
			`expected_version`: string(e.ExpectedVersion),
		},
	}

	st := status.New(codes.FailedPrecondition, e.Error())
	st, err := st.WithDetails(detailErr)
	if err != nil {
		sentry.CaptureException(err)
	}

	return st
}

// MarshalProto marshals the error to a proto.Message.
func (e *StaleObjectError) MarshalProto() proto.Message {
	return &pb.Error{
		Message: e.Error(),
		Code:    e.Code,
		Details: map[string]string{
			`kind`:             e.Kind,
			`id`:               e.ID,
			`actual_version`:   string(e.ActualVersion),
			`expected_version`: string(e.ExpectedVersion),
		},
	}
}

// MarshalJSON marshals the error to JSON.
func (e *StaleObjectError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.MarshalProto())
}

// NewStaleObjectError creates a stale object error.
func NewStaleObjectError(kind string, id string, actualVersion, expectedVersion int32, code uint32) *StaleObjectError {
	return &StaleObjectError{
		BaseError: &BaseError{
			Err: fmt.Errorf("stale object: %s/%s", kind, id),
		},
		Kind:            kind,
		ID:              id,
		ActualVersion:   actualVersion,
		ExpectedVersion: expectedVersion,
		Code:            code,
	}
}
