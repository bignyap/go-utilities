package server

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrorType represents categorized error types
type ErrorType int

const (
	ErrorInternal     ErrorType = 500
	ErrorBadRequest   ErrorType = 400
	ErrorUnauthorized ErrorType = 401
	ErrorNotFound     ErrorType = 404
	ErrorConflict     ErrorType = 409
	ErrorLargePayload ErrorType = 413
)

// PostgreSQL error codes
const (
	PgUniqueViolation     = "23505" // unique_violation
	PgForeignKeyViolation = "23503" // foreign_key_violation
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
	case ErrorConflict:
		return http.StatusConflict
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
	case ErrorConflict:
		return e.Message
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

// IsUniqueViolation checks if the error is a PostgreSQL unique constraint violation
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == PgUniqueViolation
	}
	return false
}

// IsForeignKeyViolation checks if the error is a PostgreSQL foreign key violation
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == PgForeignKeyViolation
	}
	return false
}

// GetConstraintName extracts the constraint name from a PostgreSQL error
func GetConstraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}

// ParseUniqueViolationField extracts a user-friendly field name from the constraint name
func ParseUniqueViolationField(constraintName string) string {
	// Common patterns: "tablename_fieldname_key" or "tablename_fieldname_idx"
	parts := strings.Split(constraintName, "_")
	if len(parts) >= 2 {
		// Return the field name (usually the second-to-last part before _key or _idx)
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] != "key" && parts[i] != "idx" && parts[i] != "unique" {
				return parts[i]
			}
		}
	}
	return constraintName
}
