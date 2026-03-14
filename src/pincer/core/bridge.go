package core

import "context"

// Bridge is the interface every app bridge must implement.
type Bridge interface {
	// PackageName returns the Android package name.
	PackageName() string

	// EnsureAppRunning launches the app if not already in the foreground.
	EnsureAppRunning(ctx context.Context) error

	// EnsureLoggedIn checks if the user is logged in.
	EnsureLoggedIn(ctx context.Context) (bool, error)
}

// BridgeError is a structured error returned by bridge commands.
type BridgeError struct {
	Code    string `json:"error"`
	Message string `json:"message"`
}

func (e *BridgeError) Error() string {
	return e.Message
}

// NewBridgeError creates a new BridgeError.
func NewBridgeError(code, message string) *BridgeError {
	return &BridgeError{Code: code, Message: message}
}

// Common error constructors. Each call returns a fresh instance to prevent
// shared-mutable-sentinel bugs.
var (
	ErrNotLoggedIn     = func() *BridgeError { return NewBridgeError("not_logged_in", "App requires login") }
	ErrElementNotFound = func() *BridgeError { return NewBridgeError("element_not_found", "Expected UI element not found") }
	ErrTimeout         = func() *BridgeError { return NewBridgeError("timeout", "Operation timed out") }
	ErrNavigation      = func() *BridgeError { return NewBridgeError("navigation_failed", "Could not navigate to expected screen") }
	ErrAppNotRunning   = func() *BridgeError { return NewBridgeError("app_not_running", "App is not running and could not be launched") }
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

// NewErrorResponse creates an error response from a BridgeError.
func NewErrorResponse(err *BridgeError) ErrorResponse {
	return ErrorResponse{OK: false, Error: err.Code, Message: err.Message}
}
