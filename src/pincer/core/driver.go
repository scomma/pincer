package core

import "context"

// Driver is the interface every app driver must implement.
type Driver interface {
	// PackageName returns the Android package name.
	PackageName() string

	// EnsureAppRunning launches the app if not already in the foreground.
	EnsureAppRunning(ctx context.Context) error

	// EnsureLoggedIn checks if the user is logged in.
	EnsureLoggedIn(ctx context.Context) (bool, error)
}

// DriverError is a structured error returned by driver commands.
type DriverError struct {
	Code    string `json:"error"`
	Message string `json:"message"`
}

func (e *DriverError) Error() string {
	return e.Message
}

// NewDriverError creates a new DriverError.
func NewDriverError(code, message string) *DriverError {
	return &DriverError{Code: code, Message: message}
}

// Common error constructors. Each call returns a fresh instance to prevent
// shared-mutable-sentinel bugs.
var (
	ErrNotLoggedIn     = func() *DriverError { return NewDriverError("not_logged_in", "App requires login") }
	ErrElementNotFound = func() *DriverError { return NewDriverError("element_not_found", "Expected UI element not found") }
	ErrTimeout         = func() *DriverError { return NewDriverError("timeout", "Operation timed out") }
	ErrNavigation      = func() *DriverError { return NewDriverError("navigation_failed", "Could not navigate to expected screen") }
	ErrAppNotRunning   = func() *DriverError { return NewDriverError("app_not_running", "App is not running and could not be launched") }
)

// Response is the standard JSON envelope for command output.
type Response struct {
	OK   bool `json:"ok"`
	Data any  `json:"data,omitempty"`
}

// ErrorResponse is the standard JSON envelope for errors.
type ErrorResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// NewResponse creates a success response.
func NewResponse(data any) Response {
	return Response{OK: true, Data: data}
}

// NewErrorResponse creates an error response from a DriverError.
func NewErrorResponse(err *DriverError) ErrorResponse {
	return ErrorResponse{OK: false, Error: err.Code, Message: err.Message}
}
