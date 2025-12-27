package config

import (
	"os"
	"strings"

	"github.com/bignyap/go-utilities/crypto/api"
)

// CryptoConfig holds the configuration for the crypto service
type CryptoConfig struct {
	// Provider specifies which KMS provider to use (local, vault, aws)
	Provider api.KMSProviderType

	// KeyName is the name/identifier of the encryption key in the KMS
	KeyName string
}

// VaultConfig holds HashiCorp Vault specific configuration
type VaultConfig struct {
	// Address is the Vault server address (e.g., "http://localhost:8200")
	Address string

	// Token is the authentication token
	Token string

	// TransitPath is the mount path for transit secrets engine (default: "transit")
	TransitPath string

	// KeyName is the name of the encryption key in Vault
	KeyName string

	// Namespace is the Vault namespace (for Vault Enterprise)
	Namespace string
}

// LocalConfig holds local KMS configuration (for development)
type LocalConfig struct {
	// KeyName is used as an identifier for the local key
	KeyName string
}

// AWSConfig holds AWS KMS configuration
type AWSConfig struct {
	// Region is the AWS region
	Region string

	// AccessKeyID is the AWS access key ID
	AccessKeyID string

	// SecretAccessKey is the AWS secret access key
	SecretAccessKey string

	// KeyID is the AWS KMS key ID or ARN
	KeyID string

	// Endpoint is an optional custom endpoint (for LocalStack, etc.)
	Endpoint string
}

// LoadCryptoConfig loads the general crypto configuration from environment
func LoadCryptoConfig() CryptoConfig {
	return CryptoConfig{
		Provider: api.KMSProviderType(strings.ToLower(getEnvOrDefault("CRYPTO_PROVIDER", "local"))),
		KeyName:  getEnvOrDefault("CRYPTO_KEY_NAME", "kgb-messaging-kek"),
	}
}

// LoadVaultConfig loads Vault configuration from environment variables
func LoadVaultConfig() VaultConfig {
	return VaultConfig{
		Address:     getEnvOrDefault("VAULT_ADDR", "http://localhost:8200"),
		Token:       getEnvOrDefault("VAULT_TOKEN", ""),
		TransitPath: getEnvOrDefault("VAULT_TRANSIT_PATH", "transit"),
		KeyName:     getEnvOrDefault("VAULT_KEY_NAME", "kgb-messaging-kek"),
		Namespace:   getEnvOrDefault("VAULT_NAMESPACE", ""),
	}
}

// LoadLocalConfig loads local KMS configuration from environment variables
func LoadLocalConfig() LocalConfig {
	return LocalConfig{
		KeyName: getEnvOrDefault("LOCAL_KEY_NAME", "kgb-local-kek"),
	}
}

// LoadAWSConfig loads AWS KMS configuration from environment variables
func LoadAWSConfig() AWSConfig {
	return AWSConfig{
		Region:          getEnvOrDefault("AWS_REGION", "us-east-1"),
		AccessKeyID:     getEnvOrDefault("AWS_ACCESS_KEY_ID", ""),
		SecretAccessKey: getEnvOrDefault("AWS_SECRET_ACCESS_KEY", ""),
		KeyID:           getEnvOrDefault("AWS_KMS_KEY_ID", ""),
		Endpoint:        getEnvOrDefault("AWS_KMS_ENDPOINT", ""),
	}
}

// GetKMSProviderType returns the configured KMS provider type from environment
func GetKMSProviderType() api.KMSProviderType {
	return api.KMSProviderType(strings.ToLower(getEnvOrDefault("CRYPTO_PROVIDER", "local")))
}

// DefaultCryptoConfig returns the default configuration for development
func DefaultCryptoConfig() CryptoConfig {
	return CryptoConfig{
		Provider: api.KMSProviderLocal,
		KeyName:  "kgb-local-kek",
	}
}

// DefaultVaultConfig returns the default Vault configuration for development
func DefaultVaultConfig() VaultConfig {
	return VaultConfig{
		Address:     "http://localhost:8200",
		Token:       "kgb-dev-root-token",
		TransitPath: "transit",
		KeyName:     "kgb-messaging-kek",
	}
}

// Helper function to get environment variable with default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

