package api

import "context"

// EncryptionLevel defines the encryption level for messages
type EncryptionLevel int

const (
	// EncryptionLevelNone - No encryption (plaintext storage)
	EncryptionLevelNone EncryptionLevel = 0
	// EncryptionLevelSSE - Server-Side Encryption (server encrypts before storage)
	EncryptionLevelSSE EncryptionLevel = 1
	// EncryptionLevelE2EEscrow - E2E with key escrow (recoverable by server)
	EncryptionLevelE2EEscrow EncryptionLevel = 2
	// EncryptionLevelE2EClient - True E2E encryption (client-only keys)
	EncryptionLevelE2EClient EncryptionLevel = 3
)

// KMSProviderType identifies the KMS provider type
type KMSProviderType string

const (
	KMSProviderLocal KMSProviderType = "local"
	KMSProviderVault KMSProviderType = "vault"
	KMSProviderAWS   KMSProviderType = "aws"
)

// EncryptedData holds the result of encryption
type EncryptedData struct {
	Ciphertext         []byte            // Encrypted data
	WrappedDEK         []byte            // DEK encrypted by KEK
	KeyID              string            // Key identifier for KEK
	Algorithm          string            // Encryption algorithm used
	AdditionalMetadata map[string]string // Additional metadata (nonce, version, etc.)
}

// EncryptionMetadata holds metadata about an encrypted message
type EncryptionMetadata struct {
	Algorithm  string `json:"algorithm"`
	KeyVersion int    `json:"key_version"`
	Nonce      string `json:"nonce"` // Base64 encoded nonce
	AAD        string `json:"aad"`   // Base64 encoded additional authenticated data
}

// KMSProvider defines the interface for Key Management System providers
// Implementations include local (dev), HashiCorp Vault, AWS KMS
type KMSProvider interface {
	// GenerateDEK generates a new Data Encryption Key
	// Returns plaintext DEK and wrapped (encrypted) DEK
	GenerateDEK(ctx context.Context) (plaintext []byte, wrapped []byte, err error)

	// WrapDEK wraps (encrypts) a DEK using the KEK
	WrapDEK(ctx context.Context, plaintext []byte) (wrapped []byte, err error)

	// UnwrapDEK unwraps (decrypts) a wrapped DEK using the KEK
	UnwrapDEK(ctx context.Context, wrapped []byte) (plaintext []byte, err error)

	// GetKeyID returns the current key identifier
	GetKeyID() string

	// RotateKey triggers a key rotation (creates new version)
	RotateKey(ctx context.Context) error

	// Close releases any resources held by the provider
	Close() error
}

// EncryptionService defines the interface for encryption operations
type EncryptionService interface {
	// EncryptMessage encrypts a message using envelope encryption
	// associatedData is used for authenticated encryption (AAD)
	EncryptMessage(ctx context.Context, plaintext []byte, associatedData string) (*EncryptedData, error)

	// DecryptMessage decrypts an encrypted message
	// associatedData must match what was used during encryption
	DecryptMessage(ctx context.Context, data *EncryptedData, associatedData string) ([]byte, error)

	// GetKeyID returns the current key identifier from the underlying KMS
	GetKeyID() string

	// Close releases any resources
	Close() error
}

