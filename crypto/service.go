// Package crypto provides envelope encryption services using pluggable KMS backends
package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/bignyap/go-utilities/crypto/api"
)

const (
	// Algorithm identifier for AES-256-GCM
	AlgorithmAES256GCM = "AES-256-GCM"
	// NonceSize for GCM
	NonceSize = 12
)

// Service implements envelope encryption using a KMS provider
type Service struct {
	kmsProvider api.KMSProvider
}

// NewService creates a new encryption service with the given KMS provider
func NewService(kmsProvider api.KMSProvider) *Service {
	return &Service{
		kmsProvider: kmsProvider,
	}
}

// EncryptMessage encrypts a message using envelope encryption
// 1. Generates a new DEK (Data Encryption Key) from the KMS
// 2. Encrypts the plaintext with the DEK using AES-256-GCM
// 3. Returns the ciphertext along with the wrapped DEK
func (s *Service) EncryptMessage(ctx context.Context, plaintext []byte, associatedData string) (*api.EncryptedData, error) {
	// Generate a new DEK for this message
	dek, wrappedDEK, err := s.kmsProvider.GenerateDEK(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DEK: %w", err)
	}

	// Create AES-256-GCM cipher
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt with AAD (Additional Authenticated Data)
	aad := []byte(associatedData)
	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)

	// Build metadata
	metadata := api.EncryptionMetadata{
		Algorithm:  AlgorithmAES256GCM,
		KeyVersion: 1, // Could be extracted from wrapped DEK in production
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		AAD:        base64.StdEncoding.EncodeToString(aad),
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return &api.EncryptedData{
		Ciphertext: ciphertext,
		WrappedDEK: wrappedDEK,
		KeyID:      s.kmsProvider.GetKeyID(),
		Algorithm:  AlgorithmAES256GCM,
		AdditionalMetadata: map[string]string{
			"metadata": string(metadataJSON),
		},
	}, nil
}

// DecryptMessage decrypts an encrypted message
func (s *Service) DecryptMessage(ctx context.Context, data *api.EncryptedData, associatedData string) ([]byte, error) {
	// Unwrap the DEK using KMS
	dek, err := s.kmsProvider.UnwrapDEK(ctx, data.WrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap DEK: %w", err)
	}

	// Parse metadata to get nonce
	metadataStr, ok := data.AdditionalMetadata["metadata"]
	if !ok {
		return nil, fmt.Errorf("missing encryption metadata")
	}

	var metadata api.EncryptionMetadata
	if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(metadata.Nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nonce: %w", err)
	}

	// Create AES-256-GCM cipher
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt with AAD
	aad := []byte(associatedData)
	plaintext, err := gcm.Open(nil, nonce, data.Ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// GetKeyID returns the current key identifier from the underlying KMS
func (s *Service) GetKeyID() string {
	return s.kmsProvider.GetKeyID()
}

// Close releases any resources
func (s *Service) Close() error {
	return s.kmsProvider.Close()
}

// Ensure Service implements api.EncryptionService
var _ api.EncryptionService = (*Service)(nil)

