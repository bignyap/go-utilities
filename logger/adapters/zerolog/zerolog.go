package zerolog

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"

	"github.com/bignyap/go-utilities/logger/api"
	"github.com/bignyap/go-utilities/logger/config"
	"github.com/rs/zerolog"
)

// Logger implements the Logger interface using zerolog
type Logger struct {
	log       zerolog.Logger
	component string
	fields    []api.Field
}

// NewZerologger creates a new zerolog-based logger
func NewZerologger(cfg config.LogConfig) (*Logger, error) {
	// Configure zerolog level
	level := parseLevel(cfg.Level)
	zerolog.SetGlobalLevel(level)

	// Configure output writers
	writers, err := setupWriters(cfg)
	if err != nil {
		return nil, err
	}

	var writer io.Writer
	if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = io.MultiWriter(writers...)
	}

	var logger zerolog.Logger
	if cfg.Format == "pretty" && cfg.Environment == "dev" {
		consoleWriter := zerolog.ConsoleWriter{Out: writer, TimeFormat: "15:04:05"}
		logger = zerolog.New(consoleWriter).With().Timestamp().Logger()
	} else {
		logger = zerolog.New(writer).With().Timestamp().Logger()
	}

	for k, v := range cfg.Fields {
		logger = logger.With().Interface(k, v).Logger()
	}

	return &Logger{log: logger}, nil
}

func (l *Logger) Debug(ctx context.Context, msg string, fields ...api.Field) {
	event := l.log.Debug()
	l.addContextFields(ctx, event)
	l.addFields(event, fields)
	event.Msg(msg)
}

func (l *Logger) Info(ctx context.Context, msg string, fields ...api.Field) {
	event := l.log.Info()
	l.addContextFields(ctx, event)
	l.addFields(event, fields)
	event.Msg(msg)
}

func (l *Logger) Warn(ctx context.Context, msg string, fields ...api.Field) {
	event := l.log.Warn()
	l.addContextFields(ctx, event)
	l.addFields(event, fields)
	event.Msg(msg)
}

func (l *Logger) Error(ctx context.Context, msg string, err error, fields ...api.Field) {
	event := l.log.Error()
	l.addContextFields(ctx, event)
	if err != nil {
		event = event.Err(err)
	}
	l.addFields(event, fields)
	event.Msg(msg)
}

func (l *Logger) Fatal(ctx context.Context, msg string, err error, fields ...api.Field) {
	event := l.log.Fatal()
	l.addContextFields(ctx, event)
	if err != nil {
		event = event.Err(err)
	}
	l.addFields(event, fields)
	event.Msg(msg)
}

func (l *Logger) WithTraceID(traceID string) api.Logger {
	if traceID == "" {
		return l
	}
	newLog := l.log.With().Str("trace_id", traceID).Logger()
	return l.cloneWith(newLog)
}

func (l *Logger) WithFields(fields ...api.Field) api.Logger {
	if len(fields) == 0 {
		return l
	}
	ctx := l.log.With()
	for _, f := range fields {
		ctx = ctx.Interface(f.Key, f.Value)
	}
	newLog := ctx.Logger()
	newFields := append(l.fields, fields...)
	return &Logger{log: newLog, component: l.component, fields: newFields}
}

func (l *Logger) WithComponent(component string) api.Logger {
	if component == "" {
		return l
	}
	newLog := l.log.With().Str("component", component).Logger()
	return &Logger{log: newLog, component: component, fields: l.fields}
}

func (l *Logger) ToContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, api.LoggerContextKey, l)
	if l.component != "" {
		ctx = context.WithValue(ctx, api.ComponentKey, l.component)
	}
	return ctx
}

func (l *Logger) AddField(key string, value interface{}) api.Logger {
	newLog := l.log.With().Interface(key, value).Logger()
	newFields := append(l.fields, api.Field{Key: key, Value: value})
	return &Logger{log: newLog, component: l.component, fields: newFields}
}

// addContextFields extracts trace_id and other metadata from context and adds to the log event
func (l *Logger) addContextFields(ctx context.Context, event *zerolog.Event) {
	if ctx == nil {
		return
	}
	// Extract trace_id from context
	if traceID := api.GetTraceIDFromContext(ctx); traceID != "" {
		event.Str("trace_id", traceID)
	}
}

func (l *Logger) addFields(event *zerolog.Event, fields []api.Field) {
	if l.component != "" {
		event.Str("component", l.component)
	}
	for _, f := range fields {
		event.Interface(f.Key, f.Value)
	}
}

func (l *Logger) cloneWith(newLog zerolog.Logger) *Logger {
	return &Logger{log: newLog, component: l.component, fields: l.fields}
}

func parseLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "none", "off", "silent":
		return zerolog.Disabled
	default:
		return zerolog.InfoLevel
	}
}

func setupWriters(cfg config.LogConfig) ([]io.Writer, error) {
	var writers []io.Writer
	switch cfg.Output {
	case "stdout":
		writers = append(writers, os.Stdout)
	case "file":
		// TODO: Implement file writer with rotation
	case "both":
		writers = append(writers, os.Stdout)
		// TODO: Add file writer here
	default:
		writers = append(writers, os.Stdout)
	}
	return writers, nil
}

// MemoryWriter is useful for testing
type MemoryWriter struct {
	Buffer *bytes.Buffer
}

func (m *MemoryWriter) Write(p []byte) (int, error) {
	return m.Buffer.Write(p)
}

// ErrorField returns a Field representing an error
func ErrorField(err error) api.Field {
	return api.Field{Key: "error", Value: err.Error()}
}
