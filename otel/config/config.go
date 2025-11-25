package config

import (
	"fmt"
	"os"
)

// ExporterType defines the type of exporter to use
type ExporterType string

const (
	ExporterTypeConsole    ExporterType = "console"
	ExporterTypeOTLP       ExporterType = "otlp"
	ExporterTypeElasticAPM ExporterType = "elastic-apm"
)

// SamplingType defines the type of sampling strategy
type SamplingType string

const (
	SamplingTypeAlwaysOn  SamplingType = "always-on"
	SamplingTypeAlwaysOff SamplingType = "always-off"
	SamplingTypeTraceID   SamplingType = "traceid-ratio"
)

// OtelConfig is the main configuration for OpenTelemetry
type OtelConfig struct {
	// Service resource configuration
	Resource ResourceConfig

	// Trace exporter configuration
	TraceExporter ExporterConfig

	// Metric exporter configuration
	MetricExporter ExporterConfig

	// Sampling configuration
	Sampling SamplingConfig

	// Enable/disable traces
	EnableTraces bool

	// Enable/disable metrics
	EnableMetrics bool
}

// ResourceConfig contains service resource attributes
type ResourceConfig struct {
	// ServiceName is the name of the service
	ServiceName string

	// ServiceVersion is the version of the service
	ServiceVersion string

	// ServiceEnvironment is the deployment environment (dev, staging, prod)
	ServiceEnvironment string

	// ServiceInstanceID is a unique identifier for this service instance
	ServiceInstanceID string

	// CustomAttributes are additional resource attributes
	CustomAttributes map[string]string
}

// ExporterConfig contains exporter configuration
type ExporterConfig struct {
	// Type is the exporter type (console, otlp, elastic-apm)
	Type ExporterType

	// Endpoint is the exporter endpoint (for OTLP and Elastic APM)
	Endpoint string

	// Headers are additional headers to send with exports
	Headers map[string]string

	// Insecure disables TLS for gRPC connections
	Insecure bool

	// ElasticAPMConfig contains Elastic APM specific configuration
	ElasticAPM ElasticAPMConfig
}

// ElasticAPMConfig contains Elastic APM specific configuration
type ElasticAPMConfig struct {
	// ServerURL is the Elastic APM server URL
	ServerURL string

	// SecretToken is the secret token for authentication
	SecretToken string

	// APIKey is the API key for authentication (alternative to SecretToken)
	APIKey string

	// ServiceName overrides the service name for Elastic APM
	ServiceName string

	// Environment overrides the environment for Elastic APM
	Environment string
}

// SamplingConfig contains sampling configuration
type SamplingConfig struct {
	// Type is the sampling type
	Type SamplingType

	// Ratio is the sampling ratio (0.0 to 1.0) for traceid-ratio sampling
	Ratio float64
}

// Validate validates the configuration
func (c *OtelConfig) Validate() error {
	if c.Resource.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}

	if c.EnableTraces {
		if err := c.TraceExporter.Validate(); err != nil {
			return fmt.Errorf("trace exporter config invalid: %w", err)
		}
	}

	if c.EnableMetrics {
		if err := c.MetricExporter.Validate(); err != nil {
			return fmt.Errorf("metric exporter config invalid: %w", err)
		}
	}

	if c.Sampling.Type == SamplingTypeTraceID {
		if c.Sampling.Ratio < 0 || c.Sampling.Ratio > 1 {
			return fmt.Errorf("sampling ratio must be between 0.0 and 1.0")
		}
	}

	return nil
}

// Validate validates the exporter configuration
func (e *ExporterConfig) Validate() error {
	switch e.Type {
	case ExporterTypeConsole:
		// Console exporter doesn't need additional validation
		return nil
	case ExporterTypeOTLP:
		if e.Endpoint == "" {
			return fmt.Errorf("OTLP endpoint is required")
		}
		return nil
	case ExporterTypeElasticAPM:
		if e.ElasticAPM.ServerURL == "" {
			return fmt.Errorf("Elastic APM server URL is required")
		}
		if e.ElasticAPM.SecretToken == "" && e.ElasticAPM.APIKey == "" {
			return fmt.Errorf("Elastic APM requires either secret token or API key")
		}
		return nil
	default:
		return fmt.Errorf("unknown exporter type: %s", e.Type)
	}
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() OtelConfig {
	return OtelConfig{
		Resource: ResourceConfig{
			ServiceName:        getEnv("OTEL_SERVICE_NAME", "unknown-service"),
			ServiceVersion:     getEnv("OTEL_SERVICE_VERSION", "0.0.0"),
			ServiceEnvironment: getEnv("OTEL_SERVICE_ENVIRONMENT", "development"),
			ServiceInstanceID:  getEnv("OTEL_SERVICE_INSTANCE_ID", ""),
			CustomAttributes:   make(map[string]string),
		},
		TraceExporter: ExporterConfig{
			Type:     ExporterTypeConsole,
			Insecure: true,
		},
		MetricExporter: ExporterConfig{
			Type:     ExporterTypeConsole,
			Insecure: true,
		},
		Sampling: SamplingConfig{
			Type:  SamplingTypeAlwaysOn,
			Ratio: 1.0,
		},
		EnableTraces:  true,
		EnableMetrics: true,
	}
}

// DevelopmentConfig returns a configuration optimized for development
func DevelopmentConfig() OtelConfig {
	config := DefaultConfig()
	config.Resource.ServiceEnvironment = "development"
	config.TraceExporter.Type = ExporterTypeConsole
	config.MetricExporter.Type = ExporterTypeConsole
	config.Sampling.Type = SamplingTypeAlwaysOn
	return config
}

// ProductionConfig returns a configuration optimized for production
func ProductionConfig() OtelConfig {
	config := DefaultConfig()
	config.Resource.ServiceEnvironment = "production"
	config.TraceExporter = ExporterConfig{
		Type:     ExporterTypeOTLP,
		Endpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		Insecure: getEnv("OTEL_EXPORTER_OTLP_INSECURE", "false") == "true",
	}
	config.MetricExporter = ExporterConfig{
		Type:     ExporterTypeOTLP,
		Endpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		Insecure: getEnv("OTEL_EXPORTER_OTLP_INSECURE", "false") == "true",
	}
	config.Sampling = SamplingConfig{
		Type:  SamplingTypeTraceID,
		Ratio: 0.1, // Sample 10% of traces in production
	}
	return config
}

// NewElasticAPMConfig returns a configuration for Elastic APM
func NewElasticAPMConfig() OtelConfig {
	config := DefaultConfig()
	config.TraceExporter = ExporterConfig{
		Type: ExporterTypeElasticAPM,
		ElasticAPM: ElasticAPMConfig{
			ServerURL:   getEnv("ELASTIC_APM_SERVER_URL", "http://localhost:8200"),
			SecretToken: getEnv("ELASTIC_APM_SECRET_TOKEN", ""),
			APIKey:      getEnv("ELASTIC_APM_API_KEY", ""),
			ServiceName: getEnv("ELASTIC_APM_SERVICE_NAME", ""),
			Environment: getEnv("ELASTIC_APM_ENVIRONMENT", ""),
		},
	}
	config.MetricExporter = config.TraceExporter
	return config
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
