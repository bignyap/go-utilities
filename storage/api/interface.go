package api

import (
	"context"
	"io"
)

// StorageService interface for object storage operations
// Implementations: MinIO, AWS S3
type StorageService interface {
	// Upload uploads a file to storage
	// Returns the storage path (tenant_id/object_key)
	Upload(ctx context.Context, tenantID, objectKey string, data io.Reader, size int64, contentType string) (storagePath string, err error)

	// Download downloads a file from storage
	// Returns the file data and content type
	Download(ctx context.Context, storagePath string) (data []byte, contentType string, err error)

	// GetPresignedURL generates a presigned URL for downloading
	// The URL expires after expirySeconds
	GetPresignedURL(ctx context.Context, storagePath string, expirySeconds int) (url string, err error)

	// Delete deletes a file from storage
	Delete(ctx context.Context, storagePath string) error
}

// StorageType represents the type of storage backend
type StorageType string

const (
	StorageTypeMinio StorageType = "minio"
	StorageTypeS3    StorageType = "s3"
)

