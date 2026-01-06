package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/bignyap/go-utilities/logger/api"
	"github.com/gin-gonic/gin"
)

// HTTPServer is the main implementation
type HTTPServer struct {
	config     *Config
	router     *gin.Engine
	httpServer *http.Server
	logger     api.Logger
	middleware *Middleware
	respWriter *ResponseWriter
	handlers   []Handler
	shutdownFn []func()
}

type HTTPServerOption func(*HTTPServer)

func WithLogger(logger api.Logger) HTTPServerOption {
	return func(s *HTTPServer) {
		s.logger = logger
	}
}

func WithHandler(handler Handler) HTTPServerOption {
	return func(s *HTTPServer) {
		s.handlers = append(s.handlers, handler)
	}
}

func WithMiddleware(m *Middleware) HTTPServerOption {
	return func(s *HTTPServer) {
		s.middleware = m
	}
}

func WithResponseWriter(w *ResponseWriter) HTTPServerOption {
	return func(s *HTTPServer) {
		s.respWriter = w
	}
}

func WithShutdownFunc(fn func()) HTTPServerOption {
	return func(s *HTTPServer) {
		s.shutdownFn = append(s.shutdownFn, fn)
	}
}

func NewHTTPServer(cfg *Config, opts ...HTTPServerOption) *HTTPServer {
	if cfg == nil {
		cfg = DefaultConfig(ServerHTTP)
	}

	switch cfg.Environment {
	case "prod":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	s := &HTTPServer{
		config:     cfg,
		router:     gin.New(),
		handlers:   []Handler{},
		shutdownFn: []func(){},
	}

	// s.router.RedirectTrailingSlash = false
	// s.router.RedirectFixedPath = false

	for _, opt := range opts {
		opt(s)
	}

	s.ensureDefaults()
	s.middleware.Apply(s.router)

	s.httpServer = &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: s.router,
	}

	return s
}

func (s *HTTPServer) ensureDefaults() {
	if s.logger == nil {
		s.logger = api.GetLoggerFromContext(context.Background())
		if s.logger == nil {
			s.logger = &api.DefaultLogger{}
		}
	}
	if s.middleware == nil {
		s.middleware = NewMiddleware(s.logger, s.config)
	}
	if s.respWriter == nil {
		s.respWriter = NewResponseWriter(s.logger)
	}
}

func (s *HTTPServer) Router() *gin.Engine {
	return s.router
}

func (s *HTTPServer) Start() error {
	ctx := context.Background()
	for _, h := range s.handlers {
		if err := h.Setup(s); err != nil {
			s.logger.Error(ctx, "Handler setup failed", err)
			return err
		}
	}

	s.logger.WithFields(
		api.String("port", s.config.Port),
		api.String("env", s.config.Environment),
		api.String("version", s.config.Version),
	).Info(ctx, "Starting server")

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error(ctx, "HTTP server failed", err)
		}
	}()

	return s.waitForShutdown()
}

func (s *HTTPServer) waitForShutdown() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx := context.Background()
	s.logger.Info(ctx, "Shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
	defer cancel()
	return s.Shutdown(shutdownCtx)
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	for _, fn := range s.shutdownFn {
		fn()
	}

	for _, h := range s.handlers {
		if err := h.Shutdown(); err != nil {
			s.logger.Error(ctx, "Handler shutdown error", err)
		}
	}

	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error(ctx, "Server shutdown error", err)
		return err
	}

	s.logger.Info(ctx, "Server shut down cleanly")
	return nil
}

func (s *HTTPServer) GetResponseWriter() *ResponseWriter {
	return s.respWriter
}

func (s *HTTPServer) GetLogger() api.Logger {
	return s.logger
}
