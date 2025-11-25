package api

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Provider combines TracerProvider and MeterProvider for unified OpenTelemetry access
type Provider interface {
	// Tracer returns a tracer for creating spans
	Tracer(name string, opts ...trace.TracerOption) trace.Tracer

	// Meter returns a meter for recording metrics
	Meter(name string, opts ...metric.MeterOption) metric.Meter

	// Shutdown gracefully shuts down the provider
	Shutdown(ctx context.Context) error
}

// SpanOptions contains options for creating a span
type SpanOptions struct {
	Kind       trace.SpanKind
	Attributes []attribute.KeyValue
}

// MetricType represents the type of metric
type MetricType string

const (
	MetricTypeCounter       MetricType = "counter"
	MetricTypeUpDownCounter MetricType = "updowncounter"
	MetricTypeHistogram     MetricType = "histogram"
	MetricTypeGauge         MetricType = "gauge"
)

// Common attribute helpers for consistent naming
func StringAttr(key, value string) attribute.KeyValue {
	return attribute.String(key, value)
}

func IntAttr(key string, value int) attribute.KeyValue {
	return attribute.Int(key, value)
}

func Int64Attr(key string, value int64) attribute.KeyValue {
	return attribute.Int64(key, value)
}

func Float64Attr(key string, value float64) attribute.KeyValue {
	return attribute.Float64(key, value)
}

func BoolAttr(key string, value bool) attribute.KeyValue {
	return attribute.Bool(key, value)
}

func DurationAttr(key string, value time.Duration) attribute.KeyValue {
	return attribute.Int64(key, value.Milliseconds())
}

// Common semantic conventions for HTTP
const (
	HTTPMethodKey     = "http.method"
	HTTPStatusCodeKey = "http.status_code"
	HTTPRouteKey      = "http.route"
	HTTPTargetKey     = "http.target"
	HTTPHostKey       = "http.host"
	HTTPSchemeKey     = "http.scheme"
	HTTPUserAgentKey  = "http.user_agent"
)

// Common semantic conventions for errors
const (
	ErrorTypeKey    = "error.type"
	ErrorMessageKey = "error.message"
	ErrorStackKey   = "error.stack"
)

// Common semantic conventions for service
const (
	ServiceNameKey        = "service.name"
	ServiceVersionKey     = "service.version"
	ServiceEnvironmentKey = "service.environment"
	ServiceInstanceIDKey  = "service.instance.id"
)

// SpanFromContext returns the current span from context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// ContextWithSpan returns a new context with the span
func ContextWithSpan(ctx context.Context, span trace.Span) context.Context {
	return trace.ContextWithSpan(ctx, span)
}

// RecordError records an error in the current span
func RecordError(ctx context.Context, err error, opts ...trace.EventOption) {
	if err != nil {
		span := trace.SpanFromContext(ctx)
		span.RecordError(err, opts...)
		span.SetStatus(codes.Error, err.Error())
	}
}

// SetSpanAttributes sets attributes on the current span
func SetSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}
