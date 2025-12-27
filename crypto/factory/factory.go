// Package factory provides factory functions for creating crypto services
package factory

import (
	"fmt"
	"sync"

	"github.com/bignyap/go-utilities/crypto"
	"github.com/bignyap/go-utilities/crypto/adapters/local"
	"github.com/bignyap/go-utilities/crypto/adapters/vault"
	"github.com/bignyap/go-utilities/crypto/api"
	"github.com/bignyap/go-utilities/crypto/config"
)

var (
	globalService     api.EncryptionService
	globalServiceOnce sync.Once
)

// NewKMSProvider creates a new KMS provider based on configuration
func NewKMSProvider(providerType api.KMSProviderType) (api.KMSProvider, error) {
	switch providerType {
	case api.KMSProviderLocal:
		cfg := config.LoadLocalConfig()
		return local.NewLocalKMSProvider(cfg)

	case api.KMSProviderVault:
		cfg := config.LoadVaultConfig()
		return vault.NewVaultKMSProvider(cfg)

	case api.KMSProviderAWS:
		return nil, fmt.Errorf("AWS KMS provider not yet implemented")

	default:
		return nil, fmt.Errorf("unknown KMS provider type: %s", providerType)
	}
}

// NewKMSProviderWithConfig creates a new KMS provider with explicit configuration
func NewKMSProviderWithConfig(providerType api.KMSProviderType, cfg interface{}) (api.KMSProvider, error) {
	switch providerType {
	case api.KMSProviderLocal:
		localCfg, ok := cfg.(config.LocalConfig)
		if !ok {
			return nil, fmt.Errorf("invalid config type for local provider")
		}
		return local.NewLocalKMSProvider(localCfg)

	case api.KMSProviderVault:
		vaultCfg, ok := cfg.(config.VaultConfig)
		if !ok {
			return nil, fmt.Errorf("invalid config type for vault provider")
		}
		return vault.NewVaultKMSProvider(vaultCfg)

	case api.KMSProviderAWS:
		return nil, fmt.Errorf("AWS KMS provider not yet implemented")

	default:
		return nil, fmt.Errorf("unknown KMS provider type: %s", providerType)
	}
}

// NewEncryptionService creates a new encryption service with the default configuration
func NewEncryptionService() (api.EncryptionService, error) {
	cfg := config.LoadCryptoConfig()
	return NewEncryptionServiceWithProvider(cfg.Provider)
}

// NewEncryptionServiceWithProvider creates a new encryption service with the specified provider
func NewEncryptionServiceWithProvider(providerType api.KMSProviderType) (api.EncryptionService, error) {
	provider, err := NewKMSProvider(providerType)
	if err != nil {
		return nil, fmt.Errorf("failed to create KMS provider: %w", err)
	}
	return crypto.NewService(provider), nil
}

// NewEncryptionServiceWithConfig creates a new encryption service with explicit configuration
func NewEncryptionServiceWithConfig(providerType api.KMSProviderType, cfg interface{}) (api.EncryptionService, error) {
	provider, err := NewKMSProviderWithConfig(providerType, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create KMS provider: %w", err)
	}
	return crypto.NewService(provider), nil
}

// GetGlobalService returns the global encryption service, creating it if needed
func GetGlobalService() api.EncryptionService {
	globalServiceOnce.Do(func() {
		service, err := NewEncryptionService()
		if err != nil {
			// Fallback to local provider
			fmt.Printf("Failed to create global encryption service: %v, falling back to local\n", err)
			localProvider, _ := local.NewLocalKMSProvider(config.LocalConfig{
				KeyName: "kgb-fallback-kek",
			})
			service = crypto.NewService(localProvider)
		}
		globalService = service
	})
	return globalService
}

// SetGlobalService replaces the global encryption service with the provided instance
func SetGlobalService(service api.EncryptionService) {
	if service != nil {
		globalService = service
	}
}

// Reset resets the global service to nil, forcing recreation on next call
func Reset() {
	globalService = nil
	globalServiceOnce = sync.Once{}
}

