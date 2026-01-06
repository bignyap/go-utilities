package api

import (
	"context"
	"fmt"
	"time"
)

// Logger defines the core logging interface with context-first design.
// All logging methods take context.Context as the first parameter to
// automatically extract trace_id and other request-scoped metadata.
// Implementations can wrap different logging libraries like zerolog, zap, logrus, etc.
type Logger interface {
	// Context-first logging methods - automatically extract trace_id from context
	Debug(ctx context.Context, msg string, fields ...Field)
	Info(ctx context.Context, msg string, fields ...Field)
	Warn(ctx context.Context, msg string, fields ...Field)
	Error(ctx context.Context, msg string, err error, fields ...Field)
	Fatal(ctx context.Context, msg string, err error, fields ...Field)

	// Builder methods for adding metadata
	WithTraceID(traceID string) Logger
	WithFields(fields ...Field) Logger
	WithComponent(component string) Logger
	AddField(key string, value interface{}) Logger

	// Context integration
	ToContext(ctx context.Context) context.Context
}

// Field represents a key-value pair in structured logging
type Field struct {
	Key   string
	Value interface{}
}

func (f Field) String() string {
	return fmt.Sprintf("%s=%v", f.Key, f.Value)
}

// Common field constructors
func String(key string, val string) Field {
	return Field{Key: key, Value: val}
}

func Int(key string, val int) Field {
	return Field{Key: key, Value: val}
}

func Int64(key string, val int64) Field {
	return Field{Key: key, Value: val}
}

func Duration(key string, val time.Duration) Field {
	return Field{Key: key, Value: val}
}

func Bool(key string, val bool) Field {
	return Field{Key: key, Value: val}
}

func Any(key string, val interface{}) Field {
	return Field{Key: key, Value: val}
}

// ErrorField returns a Field representing an error
func ErrorField(err error) Field {
	if err == nil {
		return Field{Key: "error", Value: nil}
	}
	return Field{Key: "error", Value: err.Error()}
}

// Standard context keys
type contextKey string

const (
	LoggerContextKey contextKey = "logger"
	TraceIDKey       contextKey = "trace-id"
	ComponentKey     contextKey = "component"
)

func GetLoggerFromContext(ctx context.Context) Logger {
	if ctx == nil {
		return nil
	}
	if logger, ok := ctx.Value(LoggerContextKey).(Logger); ok {
		return logger
	}
	return nil
}

func GetTraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// DefaultLogger is a no-op logger that satisfies the api.Logger interface
type DefaultLogger struct{}

func (d *DefaultLogger) Debug(ctx context.Context, msg string, args ...Field) {}
func (d *DefaultLogger) Info(ctx context.Context, msg string, args ...Field)  {}
func (d *DefaultLogger) Warn(ctx context.Context, msg string, args ...Field)  {}
func (d *DefaultLogger) Error(ctx context.Context, msg string, err error, args ...Field) {
}
func (d *DefaultLogger) Fatal(ctx context.Context, msg string, err error, args ...Field) {
}
func (d *DefaultLogger) WithFields(fields ...Field) Logger             { return d }
func (d *DefaultLogger) WithTraceID(traceID string) Logger             { return d }
func (d *DefaultLogger) WithComponent(component string) Logger         { return d }
func (d *DefaultLogger) AddField(key string, value interface{}) Logger { return d }
func (d *DefaultLogger) ToContext(ctx context.Context) context.Context { return ctx }
