package errors

import "fmt"

type RodentError struct {
	Code    int
	Message string
	Context string
}

const (
	ConfigNotFound = iota + 1001 // Error codes start from 1001
	ServerStart
	HealthCheck
)

var messages = map[int]string{
	ConfigNotFound: "Configuration file not found",
	ServerStart:    "Failed to start the server",
	HealthCheck:    "Health check failed",
}

func New(code int, context string) *RodentError {
	msg, ok := messages[code]
	if !ok {
		msg = "Unknown error"
	}
	return &RodentError{Code: code, Message: msg, Context: context}
}

func (e *RodentError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("Error %d: %s - %s", e.Code, e.Message, e.Context)
	}
	return fmt.Sprintf("Error %d: %s", e.Code, e.Message)
}
