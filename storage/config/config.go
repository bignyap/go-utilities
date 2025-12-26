package config

import (
	"os"
	"strings"

	"github.com/bignyap/go-utilities/storage/api"
)

// MinIOConfig holds MinIO connection configuration
type MinIOConfig struct {
	Endpoint   string
	AccessKey  string
	SecretKey  string
	BucketName string
	UseSSL     bool
}

// S3Config holds AWS S3 connection configuration
type S3Config struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	Endpoint        string // Optional: for S3-compatible services
}

// LoadMinIOConfig loads MinIO configuration from environment variables
func LoadMinIOConfig() MinIOConfig {
	return MinIOConfig{
		Endpoint:   getEnvOrDefault("MINIO_ENDPOINT", "localhost:9000"),
		AccessKey:  getEnvOrDefault("MINIO_ACCESS_KEY", "minioadmin"),
		SecretKey:  getEnvOrDefault("MINIO_SECRET_KEY", "minioadmin"),
		BucketName: getEnvOrDefault("MINIO_BUCKET", "kgb-messaging"),
		UseSSL:     getEnvOrDefault("MINIO_USE_SSL", "false") == "true",
	}
}

// LoadS3Config loads S3 configuration from environment variables
func LoadS3Config() S3Config {
	return S3Config{
		Region:          getEnvOrDefault("AWS_REGION", "us-east-1"),
		AccessKeyID:     getEnvOrDefault("AWS_ACCESS_KEY_ID", ""),
		SecretAccessKey: getEnvOrDefault("AWS_SECRET_ACCESS_KEY", ""),
		BucketName:      getEnvOrDefault("S3_BUCKET", "kgb-messaging"),
		Endpoint:        getEnvOrDefault("S3_ENDPOINT", ""), // Optional custom endpoint
	}
}

// GetStorageType returns the configured storage type from environment
func GetStorageType() api.StorageType {
	return api.StorageType(strings.ToLower(getEnvOrDefault("STORAGE_TYPE", "minio")))
}

// Helper function to get environment variable with default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

