package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bignyap/go-utilities/storage/api"
	"github.com/bignyap/go-utilities/storage/config"
)

// S3StorageService implements StorageService interface for AWS S3
type S3StorageService struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucketName    string
}

// Ensure S3StorageService implements api.StorageService
var _ api.StorageService = (*S3StorageService)(nil)

// NewS3StorageService creates a new AWS S3 storage service
func NewS3StorageService(cfg config.S3Config) (*S3StorageService, error) {
	ctx := context.Background()

	// Build AWS config options
	var awsOpts []func(*awsconfig.LoadOptions) error
	awsOpts = append(awsOpts, awsconfig.WithRegion(cfg.Region))

	// Use explicit credentials if provided
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		awsOpts = append(awsOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	// Load AWS configuration
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client options
	var s3Opts []func(*s3.Options)
	if cfg.Endpoint != "" {
		// Custom endpoint for S3-compatible services
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // Required for most S3-compatible services
		})
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg, s3Opts...)
	presignClient := s3.NewPresignClient(client)

	// Check if bucket exists (optional - might fail due to permissions)
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.BucketName),
	})
	if err != nil {
		// Log warning but don't fail - bucket might exist but we may not have HeadBucket permission
		fmt.Printf("Warning: could not verify bucket existence: %v\n", err)
	}

	return &S3StorageService{
		client:        client,
		presignClient: presignClient,
		bucketName:    cfg.BucketName,
	}, nil
}

// Upload uploads a file to S3
func (s *S3StorageService) Upload(ctx context.Context, tenantID, objectKey string, data io.Reader, size int64, contentType string) (string, error) {
	// Create storage path: tenant_id/object_key
	storagePath := fmt.Sprintf("%s/%s", tenantID, objectKey)

	// Read data into buffer for S3 SDK
	buf, err := io.ReadAll(data)
	if err != nil {
		return "", fmt.Errorf("failed to read data: %w", err)
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucketName),
		Key:           aws.String(storagePath),
		Body:          bytes.NewReader(buf),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload object: %w", err)
	}

	return storagePath, nil
}

// Download downloads a file from S3
func (s *S3StorageService) Download(ctx context.Context, storagePath string) ([]byte, string, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(storagePath),
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read object: %w", err)
	}

	contentType := ""
	if result.ContentType != nil {
		contentType = *result.ContentType
	}

	return data, contentType, nil
}

// GetPresignedURL generates a presigned URL for downloading
func (s *S3StorageService) GetPresignedURL(ctx context.Context, storagePath string, expirySeconds int) (string, error) {
	result, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(storagePath),
	}, s3.WithPresignExpires(time.Duration(expirySeconds)*time.Second))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	return result.URL, nil
}

// Delete deletes a file from S3
func (s *S3StorageService) Delete(ctx context.Context, storagePath string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(storagePath),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

