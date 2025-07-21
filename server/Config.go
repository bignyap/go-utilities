package server

import (
	"context"
	"time"

	"github.com/bignyap/go-utilities/logger/api"
	"github.com/gin-gonic/gin"
)

// Server defines the HTTP server contract
type Server interface {
	Start() error
	Router() *gin.Engine
	Shutdown(ctx context.Context) error
	GetResponseWriter() *ResponseWriter
	GetLogger() api.Logger
}

type ServerType string

const (
	ServerHTTP ServerType = "http"
	ServerGRPC ServerType = "grpc"
)

// Config defines runtime configuration
type Config struct {
	Port            string
	Environment     string
	Version         string
	MaxRequestSize  int64
	EnableProfiling bool
	ShutdownTimeout time.Duration
	ServerType      ServerType
}

func DefaultConfig(serverType ServerType) *Config {
	return &Config{
		Port:            "8080",
		Environment:     "dev",
		Version:         "dev",
		MaxRequestSize:  10 << 20, // 10 MB
		EnableProfiling: false,
		ShutdownTimeout: 15 * time.Second,
		ServerType:      serverType,
	}
}

// Handler allows for modular startup and teardown
type Handler interface {
	Setup(server Server) error
	Shutdown() error
}
