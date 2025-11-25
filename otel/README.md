# OpenTelemetry Package

A comprehensive OpenTelemetry package for distributed tracing and metrics in Go applications. This package provides a clean, reusable interface for instrumenting your KGB applications with OpenTelemetry, supporting multiple exporters including console, OTLP, and Elastic APM.

## Features

- **Distributed Tracing**: Automatic and manual span creation with context propagation
- **Metrics Collection**: Counters, histograms, and gauges for application metrics
- **Multiple Exporters**: Support for console, OTLP gRPC, and Elastic APM
- **HTTP Middleware**: Automatic instrumentation for Gin web framework
- **Clean Architecture**: Follows the same patterns as other go-utilities packages
- **Easy Configuration**: Environment variable and code-based configuration

## Installation

```bash
go get github.com/bignyap/go-utilities/otel
```

## Quick Start

### Basic Setup

```go
package main

import (
    "context"
    "log"
    
    "github.com/bignyap/go-utilities/otel/config"
    "github.com/bignyap/go-utilities/otel/factory"
)

func main() {
    // Create OpenTelemetry provider
    cfg := config.DefaultConfig()
    provider, err := factory.NewProvider(cfg)
    if err != nil {
        log.Fatalf("Failed to create provider: %v", err)
    }
    defer provider.Shutdown(context.Background())
    
    // Use the provider for tracing and metrics
    tracer := provider.Tracer("my-service")
    meter := provider.Meter("my-service")
}
```

### Configuration

#### Development Configuration (Console Output)

```go
cfg := config.DevelopmentConfig()
cfg.Resource.ServiceName = "my-service"
cfg.Resource.ServiceVersion = "1.0.0"
```

#### Production Configuration (OTLP)

```go
cfg := config.ProductionConfig()
cfg.Resource.ServiceName = "my-service"
cfg.TraceExporter.Endpoint = "localhost:4317"
cfg.MetricExporter.Endpoint = "localhost:4317"
```

#### Elastic APM Configuration

```go
cfg := config.ElasticAPMConfig()
cfg.Resource.ServiceName = "my-service"
cfg.TraceExporter.ElasticAPM.ServerURL = "http://localhost:8200"
cfg.TraceExporter.ElasticAPM.SecretToken = "your-secret-token"
```

#### Environment Variables

The package supports configuration via environment variables:

```bash
# Service information
export OTEL_SERVICE_NAME="my-service"
export OTEL_SERVICE_VERSION="1.0.0"
export OTEL_SERVICE_ENVIRONMENT="production"

# OTLP Exporter
export OTEL_EXPORTER_OTLP_ENDPOINT="localhost:4317"
export OTEL_EXPORTER_OTLP_INSECURE="true"

# Elastic APM
export ELASTIC_APM_SERVER_URL="http://localhost:8200"
export ELASTIC_APM_SECRET_TOKEN="your-secret-token"
```

## Usage Examples

### HTTP Server with Automatic Instrumentation

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/bignyap/go-utilities/otel/config"
    "github.com/bignyap/go-utilities/otel/factory"
    "github.com/bignyap/go-utilities/otel/middleware"
)

func main() {
    // Create provider
    cfg := config.DefaultConfig()
    cfg.Resource.ServiceName = "my-api"
    provider, _ := factory.NewProvider(cfg)
    defer provider.Shutdown(context.Background())
    
    // Create Gin router
    r := gin.Default()
    
    // Add OpenTelemetry middleware
    r.Use(middleware.OtelMiddleware("my-api", provider))
    r.Use(middleware.MetricsMiddleware(provider))
    
    // Define routes
    r.GET("/hello", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "Hello World"})
    })
    
    r.Run(":8080")
}
```

### Manual Span Creation

```go
func processOrder(ctx context.Context, provider api.Provider, orderID string) error {
    tracer := provider.Tracer("order-service")
    
    // Create a span
    ctx, span := tracer.Start(ctx, "processOrder",
        trace.WithSpanKind(trace.SpanKindInternal),
        trace.WithAttributes(
            api.StringAttr("order.id", orderID),
        ),
    )
    defer span.End()
    
    // Do work...
    err := validateOrder(ctx, orderID)
    if err != nil {
        // Record error in span
        span.RecordError(err)
        span.SetStatus(trace.StatusError, err.Error())
        return err
    }
    
    // Add event to span
    span.AddEvent("Order validated successfully")
    
    return nil
}
```

### Recording Metrics

```go
func recordMetrics(provider api.Provider) {
    meter := provider.Meter("my-service")
    
    // Create a counter
    requestCounter, _ := meter.Int64Counter(
        "http.requests.total",
        metric.WithDescription("Total HTTP requests"),
    )
    
    // Create a histogram
    requestDuration, _ := meter.Float64Histogram(
        "http.request.duration",
        metric.WithDescription("HTTP request duration"),
        metric.WithUnit("ms"),
    )
    
    // Record metrics
    ctx := context.Background()
    requestCounter.Add(ctx, 1,
        metric.WithAttributes(
            api.StringAttr("method", "GET"),
            api.StringAttr("path", "/api/users"),
        ),
    )
    
    requestDuration.Record(ctx, 45.2,
        metric.WithAttributes(
            api.StringAttr("method", "GET"),
            api.IntAttr("status_code", 200),
        ),
    )
}
```

### Custom Span with Helper Functions

```go
func handleRequest(ctx context.Context) error {
    // Get current span from context
    span := api.SpanFromContext(ctx)
    
    // Add attributes
    api.SetSpanAttributes(ctx,
        api.StringAttr("user.id", "12345"),
        api.StringAttr("request.type", "payment"),
    )
    
    // Add event
    api.AddSpanEvent(ctx, "Processing payment",
        api.StringAttr("amount", "100.00"),
        api.StringAttr("currency", "USD"),
    )
    
    // Record error if needed
    if err := processPayment(); err != nil {
        api.RecordError(ctx, err)
        return err
    }
    
    return nil
}
```

## Integration with Elastic APM

### Docker Compose Setup

The Elastic APM stack is included in the `kgb-dev/docker-compose.yaml`:

```bash
# Start Elastic APM stack
cd kgb-dev
docker compose up -d elasticsearch kibana apm-server

# Access Kibana UI
open http://localhost:5601
```

### Application Configuration

```go
cfg := config.ElasticAPMConfig()
cfg.Resource.ServiceName = "kgb-directory"
cfg.Resource.ServiceVersion = "1.0.0"
cfg.Resource.ServiceEnvironment = "development"

provider, err := factory.NewProvider(cfg)
if err != nil {
    log.Fatal(err)
}
defer provider.Shutdown(context.Background())
```

## Architecture

The package follows clean architecture principles:

```
otel/
├── api/              # Core interfaces and helpers
│   └── api.go
├── config/           # Configuration structures
│   └── config.go
├── adapters/         # Implementation adapters
│   └── otel/
│       └── otel_adapter.go
├── factory/          # Factory functions
│   └── factory.go
└── middleware/       # HTTP middleware
    └── middleware.go
```

## Semantic Conventions

The package provides constants for common semantic conventions:

### HTTP Attributes
- `api.HTTPMethodKey` - HTTP method
- `api.HTTPStatusCodeKey` - HTTP status code
- `api.HTTPRouteKey` - HTTP route pattern
- `api.HTTPTargetKey` - HTTP request target
- `api.HTTPHostKey` - HTTP host
- `api.HTTPSchemeKey` - HTTP scheme
- `api.HTTPUserAgentKey` - HTTP user agent

### Error Attributes
- `api.ErrorTypeKey` - Error type
- `api.ErrorMessageKey` - Error message
- `api.ErrorStackKey` - Error stack trace

### Service Attributes
- `api.ServiceNameKey` - Service name
- `api.ServiceVersionKey` - Service version
- `api.ServiceEnvironmentKey` - Service environment
- `api.ServiceInstanceIDKey` - Service instance ID

## Best Practices

1. **Always defer shutdown**: Ensure provider shutdown is called to flush pending telemetry
   ```go
   defer provider.Shutdown(context.Background())
   ```

2. **Use context propagation**: Pass context through your application to maintain trace continuity
   ```go
   ctx, span := tracer.Start(ctx, "operation")
   defer span.End()
   ```

3. **Set meaningful span names**: Use descriptive names that indicate the operation
   ```go
   ctx, span := tracer.Start(ctx, "database.query.users")
   ```

4. **Add relevant attributes**: Include contextual information in spans
   ```go
   span.SetAttributes(
       api.StringAttr("user.id", userID),
       api.IntAttr("result.count", len(results)),
   )
   ```

5. **Record errors properly**: Always record errors in spans for better debugging
   ```go
   if err != nil {
       span.RecordError(err)
       span.SetStatus(trace.StatusError, err.Error())
   }
   ```

6. **Use sampling in production**: Configure appropriate sampling to manage data volume
   ```go
   cfg.Sampling.Type = config.SamplingTypeTraceID
   cfg.Sampling.Ratio = 0.1 // Sample 10% of traces
   ```

## Troubleshooting

### No telemetry data appearing

1. Check that the provider is properly initialized
2. Verify exporter configuration (endpoint, credentials)
3. Ensure `Shutdown()` is called to flush data
4. Check network connectivity to the backend

### High memory usage

1. Reduce sampling ratio in production
2. Limit the number of attributes per span
3. Use batch exporters instead of synchronous ones

### Missing spans in distributed traces

1. Ensure context is properly propagated across service boundaries
2. Verify that all services use compatible trace context formats
3. Check that trace IDs are being correctly extracted and injected

## License

MIT License - see LICENSE file for details
