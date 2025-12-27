// Package vault provides a HashiCorp Vault Transit secrets engine KMS provider
package vault

import (
	"context"
	"encoding/base64"
	"fmt"

	vaultapi "github.com/hashicorp/vault/api"

	"github.com/bignyap/go-utilities/crypto/api"
	"github.com/bignyap/go-utilities/crypto/config"
)

const (
	// KeySize is the size of AES-256 keys in bytes
	KeySize = 32
)

// VaultKMSProvider implements KMSProvider using HashiCorp Vault Transit engine
type VaultKMSProvider struct {
	client      *vaultapi.Client
	transitPath string
	keyName     string
}

// NewVaultKMSProvider creates a new Vault KMS provider
func NewVaultKMSProvider(cfg config.VaultConfig) (*VaultKMSProvider, error) {
	// Create Vault client configuration
	vaultConfig := vaultapi.DefaultConfig()
	vaultConfig.Address = cfg.Address

	client, err := vaultapi.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	// Set the token
	client.SetToken(cfg.Token)

	// Set namespace if provided (Vault Enterprise)
	if cfg.Namespace != "" {
		client.SetNamespace(cfg.Namespace)
	}

	provider := &VaultKMSProvider{
		client:      client,
		transitPath: cfg.TransitPath,
		keyName:     cfg.KeyName,
	}

	// Verify connectivity and key existence
	if err := provider.verifyKey(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to verify Vault key: %w", err)
	}

	return provider, nil
}

// verifyKey checks that the encryption key exists in Vault
func (p *VaultKMSProvider) verifyKey(ctx context.Context) error {
	path := fmt.Sprintf("%s/keys/%s", p.transitPath, p.keyName)
	_, err := p.client.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to read key %s: %w", p.keyName, err)
	}
	return nil
}

// GenerateDEK generates a new Data Encryption Key using Vault's datakey endpoint
// This returns both the plaintext DEK and the wrapped (encrypted) DEK
func (p *VaultKMSProvider) GenerateDEK(ctx context.Context) (plaintext []byte, wrapped []byte, err error) {
	path := fmt.Sprintf("%s/datakey/plaintext/%s", p.transitPath, p.keyName)

	secret, err := p.client.Logical().WriteWithContext(ctx, path, map[string]interface{}{
		"bits": KeySize * 8, // 256 bits
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate DEK: %w", err)
	}

	// Get plaintext DEK (base64 encoded)
	plaintextB64, ok := secret.Data["plaintext"].(string)
	if !ok {
		return nil, nil, fmt.Errorf("invalid plaintext response from Vault")
	}
	plaintext, err = base64.StdEncoding.DecodeString(plaintextB64)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode plaintext DEK: %w", err)
	}

	// Get wrapped DEK (ciphertext, base64 encoded)
	wrappedB64, ok := secret.Data["ciphertext"].(string)
	if !ok {
		return nil, nil, fmt.Errorf("invalid ciphertext response from Vault")
	}
	// Store as-is (it's already the vault ciphertext format)
	wrapped = []byte(wrappedB64)

	return plaintext, wrapped, nil
}

// WrapDEK wraps (encrypts) a DEK using Vault Transit
func (p *VaultKMSProvider) WrapDEK(ctx context.Context, plaintextDEK []byte) ([]byte, error) {
	path := fmt.Sprintf("%s/encrypt/%s", p.transitPath, p.keyName)

	secret, err := p.client.Logical().WriteWithContext(ctx, path, map[string]interface{}{
		"plaintext": base64.StdEncoding.EncodeToString(plaintextDEK),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	ciphertext, ok := secret.Data["ciphertext"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid ciphertext response from Vault")
	}

	return []byte(ciphertext), nil
}

// UnwrapDEK unwraps (decrypts) a wrapped DEK using Vault Transit
func (p *VaultKMSProvider) UnwrapDEK(ctx context.Context, wrappedDEK []byte) ([]byte, error) {
	path := fmt.Sprintf("%s/decrypt/%s", p.transitPath, p.keyName)

	secret, err := p.client.Logical().WriteWithContext(ctx, path, map[string]interface{}{
		"ciphertext": string(wrappedDEK),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap DEK: %w", err)
	}

	plaintextB64, ok := secret.Data["plaintext"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid plaintext response from Vault")
	}

	plaintext, err := base64.StdEncoding.DecodeString(plaintextB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode plaintext DEK: %w", err)
	}

	return plaintext, nil
}

// GetKeyID returns the current key identifier
func (p *VaultKMSProvider) GetKeyID() string {
	return fmt.Sprintf("vault:%s/%s", p.transitPath, p.keyName)
}

// RotateKey triggers a key rotation in Vault
func (p *VaultKMSProvider) RotateKey(ctx context.Context) error {
	path := fmt.Sprintf("%s/keys/%s/rotate", p.transitPath, p.keyName)
	_, err := p.client.Logical().WriteWithContext(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("failed to rotate key: %w", err)
	}
	return nil
}

// Close releases any resources held by the provider
func (p *VaultKMSProvider) Close() error {
	// Vault client doesn't need explicit cleanup
	return nil
}

// Ensure VaultKMSProvider implements api.KMSProvider
var _ api.KMSProvider = (*VaultKMSProvider)(nil)

