package otel

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/bignyap/go-utilities/otel/api"
	"github.com/bignyap/go-utilities/otel/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc/credentials/insecure"
)

// parseEndpointURL parses a URL and returns just the host:port portion.
// If the input is not a URL (no scheme), it returns it as-is.
// This is needed because OTLP HTTP exporters expect host:port, not full URLs.
func parseEndpointURL(endpoint string) (hostPort string, isHTTPS bool) {
	// If it doesn't look like a URL, return as-is
	if !strings.Contains(endpoint, "://") {
		return endpoint, false
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return endpoint, false
	}

	isHTTPS = parsed.Scheme == "https"
	hostPort = parsed.Host
	return hostPort, isHTTPS
}

// OtelProvider implements the api.Provider interface using OpenTelemetry SDK
type OtelProvider struct {
	config         config.OtelConfig
	resource       *resource.Resource
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
}

// NewOtelProvider creates a new OpenTelemetry provider
func NewOtelProvider(cfg config.OtelConfig) (*OtelProvider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	provider := &OtelProvider{
		config: cfg,
	}

	// Create resource
	res, err := provider.createResource()
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	provider.resource = res

	// Initialize tracer provider if enabled
	if cfg.EnableTraces {
		tp, err := provider.createTracerProvider()
		if err != nil {
			return nil, fmt.Errorf("failed to create tracer provider: %w", err)
		}
		provider.tracerProvider = tp
		otel.SetTracerProvider(tp)
	}

	// Initialize meter provider if enabled
	if cfg.EnableMetrics {
		mp, err := provider.createMeterProvider()
		if err != nil {
			return nil, fmt.Errorf("failed to create meter provider: %w", err)
		}
		provider.meterProvider = mp
		otel.SetMeterProvider(mp)
	}

	return provider, nil
}

// createResource creates an OpenTelemetry resource with service metadata
func (p *OtelProvider) createResource() (*resource.Resource, error) {
	attrs := []resource.Option{
		resource.WithAttributes(
			semconv.ServiceName(p.config.Resource.ServiceName),
			semconv.ServiceVersion(p.config.Resource.ServiceVersion),
		),
	}

	// Add environment if specified
	if p.config.Resource.ServiceEnvironment != "" {
		attrs = append(attrs, resource.WithAttributes(
			semconv.DeploymentEnvironment(p.config.Resource.ServiceEnvironment),
		))
	}

	// Add instance ID if specified
	if p.config.Resource.ServiceInstanceID != "" {
		attrs = append(attrs, resource.WithAttributes(
			semconv.ServiceInstanceID(p.config.Resource.ServiceInstanceID),
		))
	}

	// Add custom attributes
	for key, value := range p.config.Resource.CustomAttributes {
		attrs = append(attrs, resource.WithAttributes(
			api.StringAttr(key, value),
		))
	}

	return resource.New(
		context.Background(),
		attrs...,
	)
}

// createTracerProvider creates a tracer provider with configured exporter
func (p *OtelProvider) createTracerProvider() (*sdktrace.TracerProvider, error) {
	exporter, err := p.createTraceExporter()
	if err != nil {
		return nil, err
	}

	// Create sampler
	sampler := p.createSampler()

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(p.resource),
		sdktrace.WithSampler(sampler),
	)

	return tp, nil
}

// createTraceExporter creates a trace exporter based on configuration
func (p *OtelProvider) createTraceExporter() (sdktrace.SpanExporter, error) {
	switch p.config.TraceExporter.Type {
	case config.ExporterTypeConsole:
		return stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)

	case config.ExporterTypeElasticAPM:
		// Elastic APM uses HTTP OTLP protocol
		// Parse URL to extract host:port (OTLP HTTP expects host:port, not full URL)
		hostPort, isHTTPS := parseEndpointURL(p.config.TraceExporter.ElasticAPM.ServerURL)

		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(hostPort),
		}

		// Add insecure option if specified or if URL uses http://
		if p.config.TraceExporter.Insecure || !isHTTPS {
			opts = append(opts, otlptracehttp.WithInsecure())
		}

		// Add headers for Elastic APM authentication
		headers := make(map[string]string)
		if p.config.TraceExporter.ElasticAPM.SecretToken != "" {
			headers["Authorization"] = "Bearer " + p.config.TraceExporter.ElasticAPM.SecretToken
		} else if p.config.TraceExporter.ElasticAPM.APIKey != "" {
			headers["Authorization"] = "ApiKey " + p.config.TraceExporter.ElasticAPM.APIKey
		}
		if len(headers) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(headers))
		}

		return otlptracehttp.New(context.Background(), opts...)

	case config.ExporterTypeOTLP:
		// Standard OTLP uses gRPC
		endpoint := p.config.TraceExporter.Endpoint

		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(endpoint),
		}

		// Add insecure option if specified
		if p.config.TraceExporter.Insecure {
			opts = append(opts, otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()))
		}

		// Add custom headers
		if len(p.config.TraceExporter.Headers) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(p.config.TraceExporter.Headers))
		}

		return otlptracegrpc.New(context.Background(), opts...)

	default:
		return nil, fmt.Errorf("unsupported trace exporter type: %s", p.config.TraceExporter.Type)
	}
}

// createMeterProvider creates a meter provider with configured exporter
func (p *OtelProvider) createMeterProvider() (*sdkmetric.MeterProvider, error) {
	exporter, err := p.createMetricExporter()
	if err != nil {
		return nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(10*time.Second),
		)),
		sdkmetric.WithResource(p.resource),
	)

	return mp, nil
}

// createMetricExporter creates a metric exporter based on configuration
func (p *OtelProvider) createMetricExporter() (sdkmetric.Exporter, error) {
	switch p.config.MetricExporter.Type {
	case config.ExporterTypeConsole:
		return stdoutmetric.New(
			stdoutmetric.WithPrettyPrint(),
		)

	case config.ExporterTypeElasticAPM:
		// Elastic APM uses HTTP OTLP protocol
		// Parse URL to extract host:port (OTLP HTTP expects host:port, not full URL)
		hostPort, isHTTPS := parseEndpointURL(p.config.MetricExporter.ElasticAPM.ServerURL)

		opts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithEndpoint(hostPort),
		}

		// Add insecure option if specified or if URL uses http://
		if p.config.MetricExporter.Insecure || !isHTTPS {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}

		// Add headers for Elastic APM authentication
		headers := make(map[string]string)
		if p.config.MetricExporter.ElasticAPM.SecretToken != "" {
			headers["Authorization"] = "Bearer " + p.config.MetricExporter.ElasticAPM.SecretToken
		} else if p.config.MetricExporter.ElasticAPM.APIKey != "" {
			headers["Authorization"] = "ApiKey " + p.config.MetricExporter.ElasticAPM.APIKey
		}
		if len(headers) > 0 {
			opts = append(opts, otlpmetrichttp.WithHeaders(headers))
		}

		return otlpmetrichttp.New(context.Background(), opts...)

	case config.ExporterTypeOTLP:
		// Standard OTLP uses gRPC
		endpoint := p.config.MetricExporter.Endpoint

		opts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(endpoint),
		}

		// Add insecure option if specified
		if p.config.MetricExporter.Insecure {
			opts = append(opts, otlpmetricgrpc.WithTLSCredentials(insecure.NewCredentials()))
		}

		// Add custom headers
		if len(p.config.MetricExporter.Headers) > 0 {
			opts = append(opts, otlpmetricgrpc.WithHeaders(p.config.MetricExporter.Headers))
		}

		return otlpmetricgrpc.New(context.Background(), opts...)

	default:
		return nil, fmt.Errorf("unsupported metric exporter type: %s", p.config.MetricExporter.Type)
	}
}

// createSampler creates a sampler based on configuration
func (p *OtelProvider) createSampler() sdktrace.Sampler {
	switch p.config.Sampling.Type {
	case config.SamplingTypeAlwaysOn:
		return sdktrace.AlwaysSample()
	case config.SamplingTypeAlwaysOff:
		return sdktrace.NeverSample()
	case config.SamplingTypeTraceID:
		return sdktrace.TraceIDRatioBased(p.config.Sampling.Ratio)
	default:
		return sdktrace.AlwaysSample()
	}
}

// Tracer returns a tracer for creating spans
func (p *OtelProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	if p.tracerProvider == nil {
		return tracenoop.NewTracerProvider().Tracer(name)
	}
	return p.tracerProvider.Tracer(name, opts...)
}

// Meter returns a meter for recording metrics
func (p *OtelProvider) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	if p.meterProvider == nil {
		return noop.NewMeterProvider().Meter(name)
	}
	return p.meterProvider.Meter(name, opts...)
}

// Shutdown gracefully shuts down the provider
func (p *OtelProvider) Shutdown(ctx context.Context) error {
	var errs []error

	if p.tracerProvider != nil {
		if err := p.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown tracer provider: %w", err))
		}
	}

	if p.meterProvider != nil {
		if err := p.meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown meter provider: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}
