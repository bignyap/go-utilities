package factory

import (
	"fmt"
	"sync"

	"github.com/bignyap/go-utilities/logger/adapters/zerolog"
	"github.com/bignyap/go-utilities/logger/api"
	"github.com/bignyap/go-utilities/logger/config"
)

var (
	globalLogger     api.Logger
	globalLoggerOnce sync.Once
)

// NewLogger creates a new logger instance based on configuration
func NewLogger(cfg config.LogConfig) (api.Logger, error) {
	// Currently we only support zerolog
	// Add more implementations by extending this
	return zerolog.NewZerologger(cfg)
}

// GetGlobalLogger returns the global logger instance, creating it if needed
func GetGlobalLogger() api.Logger {
	globalLoggerOnce.Do(func() {
		logger, err := NewLogger(config.DefaultConfig())
		if err != nil {
			// We can't use a logger to log logger creation failure
			// so we'll use fmt as a fallback
			fmt.Printf("Failed to create global logger: %v\n", err)
			// Create a minimal working logger as a fallback
			logger, _ = zerolog.NewZerologger(config.LogConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			})
		}
		globalLogger = logger
	})
	return globalLogger
}

// SetGlobalLogger replaces the global logger with the provided instance
func SetGlobalLogger(logger api.Logger) {
	if logger != nil {
		globalLogger = logger
	}
}

// Reset resets the global logger to nil, forcing recreation on next call
func Reset() {
	globalLogger = nil
	globalLoggerOnce = sync.Once{}
}
