package foundation

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/pkg/errors"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BaseError is the base error type for all errors in the Foundation framework.
type BaseError struct {
	Err error
}

func (e *BaseError) Error() string {
	return e.Err.Error()
}

// FoundationError describes an interface for all errors in the Foundation framework.
type FoundationError interface {
	error
	GRPCStatus() *status.Status
}

// GRPCStatus returns a gRPC status error for the Foundation error.
func (e *BaseError) GRPCStatus() *status.Status {
	return status.New(codes.Internal, e.Err.Error())
}

// InternalError
type InternalError struct {
	*BaseError
}

func (e *InternalError) GRPCStatus() *status.Status {
	return status.New(codes.Internal, "internal error")
}

// NewInternalError creates a generic internal error.
func NewInternalError(err error, msg string) *InternalError {
	return &InternalError{
		BaseError: &BaseError{
			Err: errors.Wrap(err, msg),
		},
	}
}

// InvalidArgumentError describes an invalid argument error.
type InvalidArgumentError struct {
	*BaseError

	Kind       string
	ID         string
	Violations map[string][]string
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
				Description: d,
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

// NewInvalidArgumentError creates an invalid argument error with error details.
func NewInvalidArgumentError(kind string, id string, violations map[string][]string) *InvalidArgumentError {
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

// UnauthenticatedError describes an unauthenticated error.
type UnauthenticatedError struct {
	*BaseError
}

func (e *UnauthenticatedError) GRPCStatus() *status.Status {
	return status.New(codes.Unauthenticated, e.Err.Error())
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

func (app *Application) foundationErrorToStatusInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	h, err := handler(ctx, req)

	if err != nil {
		if fErr, ok := err.(FoundationError); ok {
			app.ProcessError(fErr)

			return h, fErr.GRPCStatus().Err()
		}
	}

	return h, err
}

func (app *Application) ProcessError(err FoundationError) {
	// Log internal errors
	if _, ok := err.(*InternalError); ok {
		app.Logger.Error(err.Error())
		sentry.CaptureException(err)
	}
}
