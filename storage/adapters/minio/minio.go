package minio

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/bignyap/go-utilities/storage/api"
	"github.com/bignyap/go-utilities/storage/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOStorageService implements StorageService interface for MinIO
type MinIOStorageService struct {
	client     *minio.Client
	bucketName string
}

// Ensure MinIOStorageService implements api.StorageService
var _ api.StorageService = (*MinIOStorageService)(nil)

// NewMinIOStorageService creates a new MinIO storage service
func NewMinIOStorageService(cfg config.MinIOConfig) (*MinIOStorageService, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Ensure bucket exists
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		err = client.MakeBucket(ctx, cfg.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return &MinIOStorageService{
		client:     client,
		bucketName: cfg.BucketName,
	}, nil
}

// Upload uploads a file to MinIO
func (s *MinIOStorageService) Upload(ctx context.Context, tenantID, objectKey string, data io.Reader, size int64, contentType string) (string, error) {
	// Create storage path: tenant_id/object_key
	storagePath := fmt.Sprintf("%s/%s", tenantID, objectKey)

	_, err := s.client.PutObject(ctx, s.bucketName, storagePath, data, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload object: %w", err)
	}

	return storagePath, nil
}

// Download downloads a file from MinIO
func (s *MinIOStorageService) Download(ctx context.Context, storagePath string) ([]byte, string, error) {
	obj, err := s.client.GetObject(ctx, s.bucketName, storagePath, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object: %w", err)
	}
	defer obj.Close()

	// Get object info for content type
	info, err := obj.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object info: %w", err)
	}

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read object: %w", err)
	}

	return data, info.ContentType, nil
}

// GetPresignedURL generates a presigned URL for downloading
func (s *MinIOStorageService) GetPresignedURL(ctx context.Context, storagePath string, expirySeconds int) (string, error) {
	expiry := time.Duration(expirySeconds) * time.Second
	url, err := s.client.PresignedGetObject(ctx, s.bucketName, storagePath, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	return url.String(), nil
}

// Delete deletes a file from MinIO
func (s *MinIOStorageService) Delete(ctx context.Context, storagePath string) error {
	err := s.client.RemoveObject(ctx, s.bucketName, storagePath, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

