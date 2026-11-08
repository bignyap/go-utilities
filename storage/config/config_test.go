package config

import (
	"testing"
)

func TestLoadMinIOConfig(t *testing.T) {
	t.Run("Default configuration without public endpoint", func(t *testing.T) {
		t.Setenv("MINIO_ENDPOINT", "minio:9000")
		t.Setenv("MINIO_PUBLIC_ENDPOINT", "")
		t.Setenv("MINIO_USE_SSL", "false")
		t.Setenv("MINIO_PUBLIC_USE_SSL", "")

		cfg := LoadMinIOConfig()
		if cfg.Endpoint != "minio:9000" {
			t.Errorf("expected Endpoint=minio:9000, got %q", cfg.Endpoint)
		}
		if cfg.PublicEndpoint != "" {
			t.Errorf("expected empty PublicEndpoint, got %q", cfg.PublicEndpoint)
		}
		if cfg.UseSSL != false {
			t.Errorf("expected UseSSL=false, got true")
		}
		if cfg.PublicUseSSL != false {
			t.Errorf("expected PublicUseSSL=false, got true")
		}
	})

	t.Run("Configuration with public endpoint and SSL", func(t *testing.T) {
		t.Setenv("MINIO_ENDPOINT", "minio:9000")
		t.Setenv("MINIO_PUBLIC_ENDPOINT", "localhost:9000")
		t.Setenv("MINIO_USE_SSL", "false")
		t.Setenv("MINIO_PUBLIC_USE_SSL", "true")

		cfg := LoadMinIOConfig()
		if cfg.Endpoint != "minio:9000" {
			t.Errorf("expected Endpoint=minio:9000, got %q", cfg.Endpoint)
		}
		if cfg.PublicEndpoint != "localhost:9000" {
			t.Errorf("expected PublicEndpoint=localhost:9000, got %q", cfg.PublicEndpoint)
		}
		if cfg.UseSSL != false {
			t.Errorf("expected UseSSL=false, got true")
		}
		if cfg.PublicUseSSL != true {
			t.Errorf("expected PublicUseSSL=true, got false")
		}
	})
}
