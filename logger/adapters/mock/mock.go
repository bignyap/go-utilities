package mock

import (
	"context"
	"sync"

	"github.com/bignyap/go-utilities/logger/api"
)

// Mock implements the Logger interface for testing purposes
type Mock struct {
	mu             sync.Mutex
	debugMessages  []LogEntry
	infoMessages   []LogEntry
	warnMessages   []LogEntry
	errorMessages  []LogEntry
	fatalMessages  []LogEntry
	component      string
	fields         []api.Field
	traceID        string
	lastFatalError error
}

// LogEntry represents a logged message
type LogEntry struct {
	Message string
	Error   error
	Fields  []api.Field
}

// NewMockLogger creates a new mock logger
func NewMockLogger() *Mock {
	return &Mock{
		debugMessages: []LogEntry{},
		infoMessages:  []LogEntry{},
		warnMessages:  []LogEntry{},
		errorMessages: []LogEntry{},
		fatalMessages: []LogEntry{},
		fields:        []api.Field{},
	}
}

// Debug logs a debug message
func (m *Mock) Debug(msg string, fields ...api.Field) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.debugMessages = append(m.debugMessages, LogEntry{
		Message: msg,
		Fields:  fields,
	})
}

// Info logs an info message
func (m *Mock) Info(msg string, fields ...api.Field) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infoMessages = append(m.infoMessages, LogEntry{
		Message: msg,
		Fields:  fields,
	})
}

// Warn logs a warning message
func (m *Mock) Warn(msg string, fields ...api.Field) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warnMessages = append(m.warnMessages, LogEntry{
		Message: msg,
		Fields:  fields,
	})
}

// Error logs an error message
func (m *Mock) Error(msg string, err error, fields ...api.Field) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorMessages = append(m.errorMessages, LogEntry{
		Message: msg,
		Error:   err,
		Fields:  fields,
	})
}

// Fatal logs a fatal message
func (m *Mock) Fatal(msg string, err error, fields ...api.Field) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastFatalError = err
	m.fatalMessages = append(m.fatalMessages, LogEntry{
		Message: msg,
		Error:   err,
		Fields:  fields,
	})
	// Note: In a real logger this would exit the program
	// For testing we just record it
}

// WithTraceID returns a logger with trace ID set
func (m *Mock) WithTraceID(traceID string) api.Logger {
	newLogger := &Mock{
		debugMessages: m.debugMessages,
		infoMessages:  m.infoMessages,
		warnMessages:  m.warnMessages,
		errorMessages: m.errorMessages,
		fatalMessages: m.fatalMessages,
		component:     m.component,
		fields:        m.fields,
		traceID:       traceID,
	}
	return newLogger
}

// WithFields returns a logger with fields set
func (m *Mock) WithFields(fields ...api.Field) api.Logger {
	newLogger := &Mock{
		debugMessages: m.debugMessages,
		infoMessages:  m.infoMessages,
		warnMessages:  m.warnMessages,
		errorMessages: m.errorMessages,
		fatalMessages: m.fatalMessages,
		component:     m.component,
		fields:        append(m.fields, fields...),
		traceID:       m.traceID,
	}
	return newLogger
}

// WithComponent returns a logger with component name set
func (m *Mock) WithComponent(component string) api.Logger {
	newLogger := &Mock{
		debugMessages: m.debugMessages,
		infoMessages:  m.infoMessages,
		warnMessages:  m.warnMessages,
		errorMessages: m.errorMessages,
		fatalMessages: m.fatalMessages,
		component:     component,
		fields:        m.fields,
		traceID:       m.traceID,
	}
	return newLogger
}

// FromContext extracts logging information from context
func (m *Mock) FromContext(ctx context.Context) api.Logger {
	if ctx == nil {
		return m
	}

	logger := m

	if traceID := api.GetTraceIDFromContext(ctx); traceID != "" {
		logger = logger.WithTraceID(traceID).(*Mock)
	}

	if component, ok := ctx.Value(api.ComponentKey).(string); ok && component != "" {
		logger = logger.WithComponent(component).(*Mock)
	}

	return logger
}

// ToContext adds this logger to the context
func (m *Mock) ToContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	ctx = context.WithValue(ctx, api.LoggerContextKey, m)

	if m.traceID != "" {
		ctx = context.WithValue(ctx, api.TraceIDKey, m.traceID)
	}

	if m.component != "" {
		ctx = context.WithValue(ctx, api.ComponentKey, m.component)
	}

	return ctx
}

// Testing helper methods

// GetDebugMessages returns all logged debug messages
func (m *Mock) GetDebugMessages() []LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.debugMessages
}

// GetInfoMessages returns all logged info messages
func (m *Mock) GetInfoMessages() []LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.infoMessages
}

// GetWarnMessages returns all logged warning messages
func (m *Mock) GetWarnMessages() []LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.warnMessages
}

// GetErrorMessages returns all logged error messages
func (m *Mock) GetErrorMessages() []LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errorMessages
}

// GetFatalMessages returns all logged fatal messages
func (m *Mock) GetFatalMessages() []LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.fatalMessages
}

// LastFatalError returns the last fatal error
func (m *Mock) LastFatalError() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastFatalError
}

// Clear clears all logged messages
func (m *Mock) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.debugMessages = []LogEntry{}
	m.infoMessages = []LogEntry{}
	m.warnMessages = []LogEntry{}
	m.errorMessages = []LogEntry{}
	m.fatalMessages = []LogEntry{}
	m.lastFatalError = nil
}

// Clear clears all logged messages
func (m *Mock) AddField(key string, value interface{}) api.Logger {
	return m
}
