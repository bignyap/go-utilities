package factory

import (
	"context"
	"fmt"
	"sync"

	"github.com/bignyap/go-utilities/otel/adapters/otel"
	"github.com/bignyap/go-utilities/otel/api"
	"github.com/bignyap/go-utilities/otel/config"
)

var (
	globalProvider     api.Provider
	globalProviderOnce sync.Once
	globalProviderMu   sync.RWMutex
)

// NewProvider creates a new OpenTelemetry provider based on configuration
func NewProvider(cfg config.OtelConfig) (api.Provider, error) {
	return otel.NewOtelProvider(cfg)
}

// GetGlobalProvider returns the global provider instance, creating it if needed
func GetGlobalProvider() api.Provider {
	globalProviderOnce.Do(func() {
		provider, err := NewProvider(config.DefaultConfig())
		if err != nil {
			// We can't use a logger to log provider creation failure
			// so we'll use fmt as a fallback
			fmt.Printf("Failed to create global OpenTelemetry provider: %v\n", err)
			// Return a no-op provider as fallback
			provider, _ = NewProvider(config.OtelConfig{
				EnableTraces:  false,
				EnableMetrics: false,
			})
		}
		globalProviderMu.Lock()
		globalProvider = provider
		globalProviderMu.Unlock()
	})

	globalProviderMu.RLock()
	defer globalProviderMu.RUnlock()
	return globalProvider
}

// SetGlobalProvider replaces the global provider with the provided instance
func SetGlobalProvider(provider api.Provider) {
	if provider != nil {
		globalProviderMu.Lock()
		globalProvider = provider
		globalProviderMu.Unlock()
	}
}

// Shutdown gracefully shuts down the global provider
func Shutdown(ctx context.Context) error {
	globalProviderMu.RLock()
	provider := globalProvider
	globalProviderMu.RUnlock()

	if provider != nil {
		return provider.Shutdown(ctx)
	}
	return nil
}

// Reset resets the global provider to nil, forcing recreation on next call
func Reset() {
	globalProviderMu.Lock()
	globalProvider = nil
	globalProviderMu.Unlock()
	globalProviderOnce = sync.Once{}
}
