# Logger Module Architecture

## Overview

The logger module provides a flexible, structured logging system with support for context propagation, trace IDs, and multiple output formats. The design follows clean architecture principles to ensure loose coupling and testability.

## Package Structure

```
logger/
├── api/
│   └── logger.go       # Public interfaces used by client code
├── config/
│   └── config.go       # Configuration structures
├── context/
│   └── context.go      # Context utilities for logger
├── adapters/
│   ├── zerolog/        # Implementation using zerolog
│   │   └── zerolog.go
│   └── mock/           # Mock implementation for testing
│       └── mock.go
└── factory/
    └── factory.go      # Factory functions to create loggers
```

## Core Interfaces

### `Logger` Interface

The main interface that client code will use:

```go
// Logger defines the core logging interface
type Logger interface {
    // Log levels
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, err error, fields ...Field)
    Fatal(msg string, err error, fields ...Field)
    
    // Context and scoping methods
    WithTraceID(traceID string) Logger
    WithFields(fields ...Field) Logger
    WithComponent(component string) Logger
    
    // Context handling
    FromContext(ctx context.Context) Logger
    ToContext(ctx context.Context) context.Context
}
```

### `Field` Type

For structured logging:

```go
// Field represents a key-value pair in structured logging
type Field struct {
    Key   string
    Value interface{}
}

// Common field constructors
func String(key string, val string) Field
func Int(key string, val int) Field
func Duration(key string, val time.Duration) Field
func Bool(key string, val bool) Field
func Any(key string, val interface{}) Field
```

## Implementation Notes

1. **Context Integration**: The logger integrates with Go's context package for propagating log-related data through call chains.

2. **Adapter Pattern**: Different logging backends (zerolog, mock) are implemented as adapters that satisfy the Logger interface.

3. **Field-Based API**: Structured logging is enabled through a field-based API that's independent of the underlying implementation.

4. **Factory Pattern**: Logger instances are created through factory functions that handle configuration and setup details.

5. **Singleton Option**: While dependency injection is preferred, a global singleton logger is available as a convenience.

## Usage Examples

### Direct Usage

```go
// Create a logger
logger := factory.NewLogger(config.LogConfig{
    Level:       "info",
    Format:      "json",
    Output:      "stdout",
    Environment: "dev",
})

// Use the logger
logger.Info("Request received", 
    String("method", "GET"),
    String("path", "/users"),
    Duration("latency", 25*time.Millisecond),
)
```

### Context-Based Usage

```go
// Create a request context with logger
ctx := context.Background()
logger := factory.NewLogger(config.DefaultConfig())
ctx = logger.WithTraceID(uuid.New().String()).ToContext(ctx)

// Later in the request handling:
requestLogger := logger.FromContext(ctx)
requestLogger.Info("Processing request")
```

### Component-Specific Loggers

```go
// Create a component-specific logger
dbLogger := logger.WithComponent("database")
dbLogger.Debug("Running query", String("query", "SELECT * FROM users"))
```
