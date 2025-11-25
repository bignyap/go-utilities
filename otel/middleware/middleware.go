package middleware

import (
	"fmt"
	"time"

	"github.com/bignyap/go-utilities/otel/api"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// OtelMiddleware returns a Gin middleware that automatically instruments HTTP requests
func OtelMiddleware(serviceName string, provider api.Provider) gin.HandlerFunc {
	// Use the otelgin middleware for automatic instrumentation
	return otelgin.Middleware(serviceName)
}

// OtelMiddlewareWithConfig returns a Gin middleware with custom configuration
func OtelMiddlewareWithConfig(serviceName string, provider api.Provider, opts ...otelgin.Option) gin.HandlerFunc {
	return otelgin.Middleware(serviceName, opts...)
}

// CustomSpanMiddleware creates a custom span for each request with additional attributes
func CustomSpanMiddleware(provider api.Provider) gin.HandlerFunc {
	return func(c *gin.Context) {
		tracer := provider.Tracer("gin-http-server")

		ctx, span := tracer.Start(c.Request.Context(), fmt.Sprintf("%s %s", c.Request.Method, c.FullPath()),
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String(api.HTTPMethodKey, c.Request.Method),
				attribute.String(api.HTTPRouteKey, c.FullPath()),
				attribute.String(api.HTTPTargetKey, c.Request.URL.Path),
				attribute.String(api.HTTPHostKey, c.Request.Host),
				attribute.String(api.HTTPSchemeKey, c.Request.URL.Scheme),
				attribute.String(api.HTTPUserAgentKey, c.Request.UserAgent()),
			),
		)
		defer span.End()

		// Store the context with span in the Gin context
		c.Request = c.Request.WithContext(ctx)

		// Process request
		c.Next()

		// Add response status code
		span.SetAttributes(attribute.Int(api.HTTPStatusCodeKey, c.Writer.Status()))

		// Record errors if any
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				span.RecordError(err.Err)
			}
			span.SetStatus(codes.Error, c.Errors.String())
		}
	}
}

// MetricsMiddleware records HTTP metrics for each request
func MetricsMiddleware(provider api.Provider) gin.HandlerFunc {
	meter := provider.Meter("gin-http-server")

	// Create metrics
	requestCounter, _ := meter.Int64Counter(
		"http.server.requests",
		metric.WithDescription("Total number of HTTP requests"),
	)

	requestDuration, _ := meter.Float64Histogram(
		"http.server.duration",
		metric.WithDescription("HTTP request duration in milliseconds"),
		metric.WithUnit("ms"),
	)

	activeRequests, _ := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of active HTTP requests"),
	)

	return func(c *gin.Context) {
		// Increment active requests
		activeRequests.Add(c.Request.Context(), 1,
			metric.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.route", c.FullPath()),
			),
		)

		// Record start time
		start := time.Now()

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start).Milliseconds()

		// Common attributes
		attrs := metric.WithAttributes(
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.route", c.FullPath()),
			attribute.Int("http.status_code", c.Writer.Status()),
		)

		// Record metrics
		requestCounter.Add(c.Request.Context(), 1, attrs)
		requestDuration.Record(c.Request.Context(), float64(duration), attrs)

		// Decrement active requests
		activeRequests.Add(c.Request.Context(), -1,
			metric.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.route", c.FullPath()),
			),
		)
	}
}
