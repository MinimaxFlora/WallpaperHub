// Package errs defines the structured error type rendered by the HTTP API.
package errs

import "fmt"

// Error is an API error carrying an HTTP status, a stable machine-readable
// code and a human-readable message.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// New builds an Error.
func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// BadRequest reports a malformed request parameter.
func BadRequest(code, message string) *Error { return New(400, code, message) }

// Forbidden reports a request rejected by the hotlink allowlist.
func Forbidden(code, message string) *Error { return New(403, code, message) }

// NotFound reports a missing resource or an empty match set.
func NotFound(code, message string) *Error { return New(404, code, message) }

// TooMany reports a rate limit violation.
func TooMany(code, message string) *Error { return New(429, code, message) }

// Unavailable reports that a dependency such as the manifest is not ready.
func Unavailable(code, message string) *Error { return New(503, code, message) }
