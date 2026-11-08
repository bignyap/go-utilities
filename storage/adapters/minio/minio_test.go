package minio

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestPresignedURL_DoesNotContactPublicEndpointWhenRegionSet(t *testing.T) {
	// A completely unreachable endpoint that would fail if any HTTP request is attempted
	publicEndpoint := "127.0.0.1:59999"

	// 1. Without region set: PresignedGetObject tries to fetch bucket location and fails!
	clientWithoutRegion, err := minio.New(publicEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, errWithoutRegion := clientWithoutRegion.PresignedGetObject(context.Background(), "my-bucket", "test.jpg", 1*time.Hour, nil)
	if errWithoutRegion == nil {
		t.Fatalf("expected error without region because it tries to call location API, but got nil")
	}
	t.Logf("Confirmed failure without region: %v", errWithoutRegion)

	// 2. WITH region set: PresignedGetObject succeeds locally with zero network calls!
	clientWithRegion, err := minio.New(publicEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("failed to create client with region: %v", err)
	}

	presignedURL, errWithRegion := clientWithRegion.PresignedGetObject(context.Background(), "my-bucket", "test.jpg", 1*time.Hour, nil)
	if errWithRegion != nil {
		t.Fatalf("expected success with region set, got: %v", errWithRegion)
	}

	urlStr := presignedURL.String()
	t.Logf("Generated presigned URL: %s", urlStr)
	if !strings.HasPrefix(urlStr, "http://127.0.0.1:59999/my-bucket/test.jpg") {
		t.Fatalf("unexpected URL prefix: %s", urlStr)
	}
	if !strings.Contains(urlStr, "X-Amz-Signature=") {
		t.Fatalf("expected signature in URL: %s", urlStr)
	}
}
