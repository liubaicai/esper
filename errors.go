package esper

import "fmt"

// ErrorCode is the stable, machine-readable category used across Build,
// Deploy, runtime and connector boundaries.
type ErrorCode string

const (
	ErrorInvalidRule  ErrorCode = "InvalidRule"
	ErrorTypeMismatch ErrorCode = "TypeMismatch"
	ErrorUnknownName  ErrorCode = "UnknownName"
	ErrorAmbiguous    ErrorCode = "Ambiguous"
	ErrorDependency   ErrorCode = "Dependency"
	ErrorDeployment   ErrorCode = "Deployment"
	ErrorState        ErrorCode = "State"
	ErrorTimeout      ErrorCode = "Timeout"
	ErrorCanceled     ErrorCode = "Canceled"
	ErrorConnector    ErrorCode = "Connector"
	ErrorInternal     ErrorCode = "Internal"
)

func (c ErrorCode) Error() string { return string(c) }

type Error struct {
	Code    ErrorCode
	Path    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return "esper: <nil error>"
	}
	if e.Path == "" {
		return fmt.Sprintf("esper: %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("esper: %s at %s: %s", e.Code, e.Path, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	code, ok := target.(ErrorCode)
	return ok && e.Code == code
}

func NewError(code ErrorCode, message string) error {
	return &Error{Code: code, Message: message}
}

func WrapError(code ErrorCode, path string, cause error) error {
	if cause == nil {
		return nil
	}
	if typed, ok := cause.(*Error); ok && typed.Code == code {
		return cause
	}
	return &Error{Code: code, Path: path, Message: cause.Error(), Cause: cause}
}
