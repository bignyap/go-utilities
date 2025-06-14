package server

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
)

// ErrorType represents categorized error types
type ErrorType int

const (
	ErrorInternal     ErrorType = 500
	ErrorBadRequest   ErrorType = 400
	ErrorUnauthorized ErrorType = 401
	ErrorNotFound     ErrorType = 404
	ErrorLargePayload ErrorType = 413
)

// InternalError wraps errors with context
type InternalError struct {
	Type       ErrorType
	Message    string
	Original   error
	CallerInfo string
}

func (e *InternalError) Error() string {
	if e.Original != nil {
		return fmt.Sprintf("[%d] %s (at %s): %v", e.Type, e.Message, e.CallerInfo, e.Original)
	}
	return fmt.Sprintf("[%d] %s (at %s)", e.Type, e.Message, e.CallerInfo)
}

func (e *InternalError) Unwrap() error {
	return e.Original
}

type ApiError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message"`
	TraceID string `json:"trace_id"`
}

func (e *ApiError) Error() string {
	return fmt.Sprintf("[%s] %s", e.TraceID, e.Message)
}

// NewError creates a new structured internal error
func NewError(errType ErrorType, message string, err error) *InternalError {
	return &InternalError{
		Type:       errType,
		Message:    message,
		Original:   err,
		CallerInfo: captureCallerInfo(2),
	}
}

func (e *InternalError) ToHttpStatusCode() int {
	switch e.Type {
	case ErrorBadRequest:
		return http.StatusBadRequest
	case ErrorUnauthorized:
		return http.StatusUnauthorized
	case ErrorNotFound:
		return http.StatusNotFound
	case ErrorLargePayload:
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}

func (e *InternalError) ToHttpMessage() string {
	switch e.Type {
	case ErrorBadRequest:
		return e.Message
	case ErrorUnauthorized:
		return "Unauthorized"
	case ErrorNotFound:
		return "Not found"
	case ErrorLargePayload:
		return "Payload too large"
	default:
		return "Internal server error"
	}
}

// ToApiError converts error to API-safe structure
func ToApiError(c *gin.Context, err error) *ApiError {
	traceID := getTraceIDFromContext(c)

	switch e := err.(type) {
	case *ApiError:
		if e.TraceID == "" {
			e.TraceID = traceID
		}
		return e
	case *InternalError:
		return &ApiError{
			Code:    e.ToHttpStatusCode(),
			Message: e.ToHttpMessage(),
			TraceID: traceID,
		}
	default:
		return &ApiError{
			Code:    http.StatusInternalServerError,
			Message: "Internal server error",
			TraceID: traceID,
		}
	}
}

func captureCallerInfo(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown:0"
	}
	return fmt.Sprintf("%s:%d", file, line)
}
