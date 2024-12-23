package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Create sentinel errors for common cases
var (
	ErrZFSPoolPropertyNotFound = &RodentError{
		Code:       ZFSPoolPropertyNotFound,
		Domain:     DomainZFS,
		Message:    errorDefinitions[ZFSPoolPropertyNotFound].message,
		HTTPStatus: errorDefinitions[ZFSPoolPropertyNotFound].httpStatus,
	}

	ErrZFSDatasetPropertyNotFound = &RodentError{
		Code:       ZFSDatasetPropertyNotFound,
		Domain:     DomainZFS,
		Message:    errorDefinitions[ZFSDatasetPropertyNotFound].message,
		HTTPStatus: errorDefinitions[ZFSDatasetPropertyNotFound].httpStatus,
	}
)

func (e *RodentError) Error() string {
	// The reason Error() doesn't include metadata is that:
	// - It follows the standard error interface pattern for concise error messages
	// - Metadata is meant for structured data consumption (API responses, logging, monitoring)
	// - Including all metadata would make error messages too verbose for standard logging
	if e.Details != "" {
		return fmt.Sprintf("[%s-%d] %s - %s", e.Domain, e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s-%d] %s", e.Domain, e.Code, e.Message)
}

func (e *RodentError) WithMetadata(key, value string) *RodentError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]string)
	}
	e.Metadata[key] = value
	return e
}

// MarshalJSON customizes JSON serialization
func (e *RodentError) MarshalJSON() ([]byte, error) {
	type Alias RodentError
	return json.Marshal(&struct {
		*Alias
		Timestamp string `json:"timestamp"`
	}{
		Alias:     (*Alias)(e),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// New creates a new RodentError
func New(code ErrorCode, details string) *RodentError {
	def, ok := errorDefinitions[code]
	if !ok {
		return &RodentError{
			Code:       code,
			Domain:     "UNKNOWN",
			Message:    "Unknown error",
			Details:    details,
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	return &RodentError{
		Code:       code,
		Domain:     def.domain,
		Message:    def.message,
		Details:    details,
		HTTPStatus: def.httpStatus,
	}
}

// Is implements the interface for errors.Is
func (e *RodentError) Is(target error) bool {
	if t, ok := target.(*RodentError); ok {
		// Match by error code within the same domain
		return e.Code == t.Code && e.Domain == t.Domain
	}
	return false
}

// Is checks if an error matches a sentinel error
func Is(err, target error) bool {
	re, ok := err.(*RodentError)
	if !ok {
		return false
	}

	if t, ok := target.(*RodentError); ok {
		return re.Code == t.Code && re.Domain == t.Domain
	}
	return false
}

// Wrap wraps an existing error with additional context
func Wrap(err error, code ErrorCode) *RodentError {
	re := New(code, err.Error())
	re.WithMetadata("wrapped_error", err.Error())
	return re
}

// Unwrap implements the interface for errors.Unwrap
func (e *RodentError) Unwrap() error {
	// If this error was created via Wrap(), return the original error
	if e.Metadata != nil {
		if originalErr, ok := e.Metadata["wrapped_error"]; ok {
			return fmt.Errorf(originalErr)
		}
	}
	return nil
}

// IsRodentError checks if an error is a RodentError
func IsRodentError(err error) bool {
	_, ok := err.(*RodentError)
	return ok
}

// CommandError helper for command execution errors
type CommandError struct {
	Command  string
	ExitCode int
	StdErr   string
}

func NewCommandError(cmd string, exitCode int, stderr string) *RodentError {
	return New(CommandExecution, "").
		WithMetadata("command", cmd).
		WithMetadata("exit_code", fmt.Sprintf("%d", exitCode)).
		WithMetadata("stderr", stderr)
}
