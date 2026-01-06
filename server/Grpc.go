package server

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/bignyap/go-utilities/logger/api"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type GRPCServer struct {
	config     *Config
	grpcServer *grpc.Server
	logger     api.Logger
	handlers   []Handler
	shutdownFn []func()
}

func NewGRPCServer(cfg *Config, opts ...HTTPServerOption) *GRPCServer {
	s := &GRPCServer{
		config:     cfg,
		grpcServer: grpc.NewServer(),
		shutdownFn: []func(){},
	}

	for _, opt := range opts {
		// Optional: adapt or define new GRPC options
		opt(&HTTPServer{
			logger:     s.logger,
			handlers:   s.handlers,
			shutdownFn: s.shutdownFn,
		})
	}

	if s.logger == nil {
		s.logger = api.GetLoggerFromContext(context.Background())
		if s.logger == nil {
			s.logger = &api.DefaultLogger{}
		}
	}

	return s
}

func (s *GRPCServer) Start() error {
	ctx := context.Background()
	for _, h := range s.handlers {
		if err := h.Setup(nil); err != nil {
			s.logger.Error(ctx, "gRPC handler setup failed", err)
			return err
		}
	}

	lis, err := net.Listen("tcp", ":"+s.config.Port)
	if err != nil {
		s.logger.Error(ctx, "Failed to listen", err)
		return err
	}

	s.logger.WithFields(
		api.String("port", s.config.Port),
		api.String("env", s.config.Environment),
		api.String("version", s.config.Version),
	).Info(ctx, "Starting gRPC server")

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.Error(ctx, "gRPC server failed", err)
		}
	}()

	return s.waitForShutdown()
}

func (s *GRPCServer) waitForShutdown() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx := context.Background()
	s.logger.Info(ctx, "Shutdown signal received for gRPC")

	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
	defer cancel()
	return s.Shutdown(shutdownCtx)
}

func (s *GRPCServer) Shutdown(ctx context.Context) error {
	for _, fn := range s.shutdownFn {
		fn()
	}
	for _, h := range s.handlers {
		if err := h.Shutdown(); err != nil {
			s.logger.Error(ctx, "Handler shutdown error", err)
		}
	}
	s.grpcServer.GracefulStop()
	s.logger.Info(ctx, "gRPC server shut down cleanly")
	return nil
}

func (s *GRPCServer) GetLogger() api.Logger {
	return s.logger
}

// gRPC has no response writer, so so panic
func (s *GRPCServer) GetResponseWriter() *ResponseWriter {
	panic("GetResponseWriter() not supported in GRPCServer")
}

// gRPC has no router, so panic
func (s *GRPCServer) Router() *gin.Engine {
	panic("Router() not supported in GRPCServer")
}
