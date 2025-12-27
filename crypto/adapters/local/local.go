// Package local provides a local in-memory KMS provider for development
// WARNING: This should NOT be used in production as keys are stored in memory
// and will be lost on restart
package local

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"sync"

	"github.com/bignyap/go-utilities/crypto/api"
	"github.com/bignyap/go-utilities/crypto/config"
)

const (
	// KeySize is the size of AES-256 keys in bytes
	KeySize = 32
	// NonceSize is the size of GCM nonce in bytes
	NonceSize = 12
)

// LocalKMSProvider implements KMSProvider for local development
// Keys are stored in memory and will be lost on restart
type LocalKMSProvider struct {
	mu         sync.RWMutex
	kek        []byte // Key Encryption Key (for wrapping DEKs)
	keyName    string
	keyVersion int
}

// NewLocalKMSProvider creates a new local KMS provider
// A new KEK is generated on each instantiation
func NewLocalKMSProvider(cfg config.LocalConfig) (*LocalKMSProvider, error) {
	kek := make([]byte, KeySize)
	if _, err := rand.Read(kek); err != nil {
		return nil, fmt.Errorf("failed to generate KEK: %w", err)
	}

	return &LocalKMSProvider{
		kek:        kek,
		keyName:    cfg.KeyName,
		keyVersion: 1,
	}, nil
}

// GenerateDEK generates a new Data Encryption Key
// Returns both the plaintext DEK and the wrapped (encrypted) DEK
func (p *LocalKMSProvider) GenerateDEK(ctx context.Context) (plaintext []byte, wrapped []byte, err error) {
	// Generate a new DEK
	dek := make([]byte, KeySize)
	if _, err := rand.Read(dek); err != nil {
		return nil, nil, fmt.Errorf("failed to generate DEK: %w", err)
	}

	// Wrap the DEK using the KEK
	wrappedDEK, err := p.WrapDEK(ctx, dek)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	return dek, wrappedDEK, nil
}

// WrapDEK wraps (encrypts) a DEK using the KEK with AES-256-GCM
func (p *LocalKMSProvider) WrapDEK(ctx context.Context, plaintext []byte) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	block, err := aes.NewCipher(p.kek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal: nonce is prepended to ciphertext
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// UnwrapDEK unwraps (decrypts) a wrapped DEK using the KEK
func (p *LocalKMSProvider) UnwrapDEK(ctx context.Context, wrapped []byte) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	block, err := aes.NewCipher(p.kek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(wrapped) < nonceSize {
		return nil, fmt.Errorf("wrapped DEK too short")
	}

	nonce, ciphertext := wrapped[:nonceSize], wrapped[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt DEK: %w", err)
	}

	return plaintext, nil
}

// GetKeyID returns the current key identifier
func (p *LocalKMSProvider) GetKeyID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return fmt.Sprintf("%s:v%d", p.keyName, p.keyVersion)
}

// RotateKey generates a new KEK version
// Note: This is simplified for development; real key rotation would need
// to handle re-encryption of existing wrapped DEKs
func (p *LocalKMSProvider) RotateKey(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	newKEK := make([]byte, KeySize)
	if _, err := rand.Read(newKEK); err != nil {
		return fmt.Errorf("failed to generate new KEK: %w", err)
	}

	p.kek = newKEK
	p.keyVersion++
	return nil
}

// Close releases any resources held by the provider
func (p *LocalKMSProvider) Close() error {
	return nil
}

// Ensure LocalKMSProvider implements api.KMSProvider
var _ api.KMSProvider = (*LocalKMSProvider)(nil)

