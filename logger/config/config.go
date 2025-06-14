package config

import "io"

// LogConfig defines all configuration options for loggers
type LogConfig struct {
	// Level is the minimum log level (debug, info, warn, error, none)
	Level string

	// Format determines the output format (json, console, pretty)
	Format string

	// Output determines where logs are written (stdout, file, both)
	Output string

	// Environment affects logging behavior (dev, test, prod)
	Environment string

	// FileOptions contains file-specific logging options
	FileOptions FileOptions

	// Fields contains default fields to add to all log messages
	Fields map[string]interface{}
}

// FileOptions configures file-based logging
type FileOptions struct {
	// Directory where log files will be stored
	Directory string

	// Filename for log files
	Filename string

	// MaxSize in megabytes before rotating
	MaxSize int

	// MaxBackups is the maximum number of old log files to retain
	MaxBackups int

	// MaxAge is the maximum number of days to retain old log files
	MaxAge int

	// Compress determines if rotated logs should be compressed
	Compress bool
}

// Writers returns the appropriate writers based on the configuration
func (c *LogConfig) Writers() ([]io.Writer, error) {
	// Implementation would set up the requested writers
	// (stdout, files, etc.) based on configuration
	return nil, nil
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() LogConfig {
	return LogConfig{
		Level:       "info",
		Format:      "json",
		Output:      "stdout",
		Environment: "dev",
		FileOptions: FileOptions{
			Directory:  "./logs",
			Filename:   "application.log",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		},
		Fields: map[string]interface{}{},
	}
}

// DevelopmentConfig returns a configuration optimized for development
func DevelopmentConfig() LogConfig {
	config := DefaultConfig()
	config.Level = "debug"
	config.Format = "pretty"
	return config
}

// ProductionConfig returns a configuration optimized for production
func ProductionConfig() LogConfig {
	config := DefaultConfig()
	config.Level = "info"
	config.Output = "both"
	config.Environment = "prod"
	return config
}
