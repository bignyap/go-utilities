package initialize

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/bignyap/go-utilities/otel/api"
	"github.com/bignyap/go-utilities/otel/config"
	"github.com/bignyap/go-utilities/otel/factory"
)

// TelemetryConfig holds telemetry configuration options
type TelemetryConfig struct {
	// ServiceName is the name of the service for telemetry identification
	ServiceName string
	// DefaultEnabled determines if telemetry is enabled by default when env vars are not set
	DefaultEnabled bool
}

// InitializeTelemetryFromEnv creates a telemetry provider from environment variables.
// It reads the following environment variables:
//   - OTEL_ENABLE_TRACES: Enable distributed tracing (default: false or TelemetryConfig.DefaultEnabled)
//   - OTEL_ENABLE_METRICS: Enable metrics collection (default: false or TelemetryConfig.DefaultEnabled)
//   - OTEL_SERVICE_NAME: Service name for telemetry (default: TelemetryConfig.ServiceName)
//   - OTEL_SERVICE_VERSION: Service version (default: "1.0.0")
//   - OTEL_SERVICE_ENVIRONMENT: Environment name (default: "dev")
//   - OTEL_SAMPLING_TYPE: Sampling type - "traceid" or "always" (default: "traceid")
//   - OTEL_SAMPLING_RATIO: Sampling ratio 0.0-1.0 (default: 1.0)
//   - ELASTIC_APM_SERVER_URL: Elastic APM server URL (default: "http://apm-server:8200")
//   - ELASTIC_APM_SECRET_TOKEN: Elastic APM secret token (default: "")
//
// Returns nil provider if both traces and metrics are disabled.
func InitializeTelemetryFromEnv(cfg TelemetryConfig) (api.Provider, error) {
	defaultEnabled := boolToString(cfg.DefaultEnabled)

	enableTraces, _ := strconv.ParseBool(getEnvOrDefault("OTEL_ENABLE_TRACES", defaultEnabled))
	enableMetrics, _ := strconv.ParseBool(getEnvOrDefault("OTEL_ENABLE_METRICS", defaultEnabled))

	if !enableTraces && !enableMetrics {
		// Telemetry is disabled
		return nil, nil
	}

	// Build OpenTelemetry configuration
	otelCfg := config.OtelConfig{
		EnableTraces:  enableTraces,
		EnableMetrics: enableMetrics,
		Resource: config.ResourceConfig{
			ServiceName:        getEnvOrDefault("OTEL_SERVICE_NAME", cfg.ServiceName),
			ServiceVersion:     getEnvOrDefault("OTEL_SERVICE_VERSION", "1.0.0"),
			ServiceEnvironment: getEnvOrDefault("OTEL_SERVICE_ENVIRONMENT", "dev"),
		},
	}

	// Configure trace exporter if traces are enabled
	if enableTraces {
		samplingRatio, _ := strconv.ParseFloat(getEnvOrDefault("OTEL_SAMPLING_RATIO", "1.0"), 64)
		otelCfg.Sampling = config.SamplingConfig{
			Type:  config.SamplingType(getEnvOrDefault("OTEL_SAMPLING_TYPE", "traceid")),
			Ratio: samplingRatio,
		}

		otelCfg.TraceExporter = config.ExporterConfig{
			Type:     config.ExporterTypeElasticAPM,
			Insecure: true, // APM server is typically in the same Docker network
			ElasticAPM: config.ElasticAPMConfig{
				ServerURL:   getEnvOrDefault("ELASTIC_APM_SERVER_URL", "http://apm-server:8200"),
				SecretToken: getEnvOrDefault("ELASTIC_APM_SECRET_TOKEN", ""),
			},
		}
	}

	// Configure metric exporter if metrics are enabled
	if enableMetrics {
		otelCfg.MetricExporter = config.ExporterConfig{
			Type:     config.ExporterTypeElasticAPM,
			Insecure: true,
			ElasticAPM: config.ElasticAPMConfig{
				ServerURL:   getEnvOrDefault("ELASTIC_APM_SERVER_URL", "http://apm-server:8200"),
				SecretToken: getEnvOrDefault("ELASTIC_APM_SECRET_TOKEN", ""),
			},
		}
	}

	// Create the OpenTelemetry provider
	provider, err := factory.NewProvider(otelCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry provider: %w", err)
	}

	return provider, nil
}

// ShutdownTelemetry gracefully shuts down the telemetry provider.
// It's safe to call with a nil provider.
func ShutdownTelemetry(provider api.Provider) error {
	if provider == nil {
		return nil
	}

	ctx := context.Background()
	if err := provider.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown telemetry provider: %w", err)
	}

	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

