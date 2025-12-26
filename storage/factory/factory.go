package factory

import (
	"fmt"

	minioadapter "github.com/bignyap/go-utilities/storage/adapters/minio"
	s3adapter "github.com/bignyap/go-utilities/storage/adapters/s3"
	"github.com/bignyap/go-utilities/storage/api"
	"github.com/bignyap/go-utilities/storage/config"
)

// NewStorageService creates a storage service based on the STORAGE_TYPE environment variable
// Supported types: "minio" (default), "s3"
func NewStorageService() (api.StorageService, error) {
	storageType := config.GetStorageType()
	return NewStorageServiceWithType(storageType)
}

// NewStorageServiceWithType creates a storage service of a specific type
// Useful for testing or when you need to explicitly specify the type
func NewStorageServiceWithType(storageType api.StorageType) (api.StorageService, error) {
	switch storageType {
	case api.StorageTypeMinio:
		cfg := config.LoadMinIOConfig()
		return minioadapter.NewMinIOStorageService(cfg)

	case api.StorageTypeS3:
		cfg := config.LoadS3Config()
		return s3adapter.NewS3StorageService(cfg)

	default:
		return nil, fmt.Errorf("unsupported storage type: %s (supported: minio, s3)", storageType)
	}
}

