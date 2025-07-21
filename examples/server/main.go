package main

import (
	"fmt"

	"github.com/bignyap/go-utilities/logger/api"
	"github.com/bignyap/go-utilities/server"
	"github.com/gin-gonic/gin"
)

// SampleHandler demonstrates a basic route handler using ResponseWriter
type SampleHandler struct {
	log    api.Logger
	writer *server.ResponseWriter
}

func NewSampleHandler(log api.Logger) *SampleHandler {
	return &SampleHandler{log: log}
}

func (h *SampleHandler) Setup(s server.Server) error {
	h.writer = s.GetResponseWriter()
	router := s.Router()
	router.GET("/ping", h.handlePing)
	router.GET("/error", h.handleError)
	return nil
}

func (h *SampleHandler) Shutdown() error {
	h.log.Info("Shutting down SampleHandler")
	return nil
}

func (h *SampleHandler) handlePing(c *gin.Context) {
	h.writer.Success(c, gin.H{"message": "pong"})
}

func (h *SampleHandler) handleError(c *gin.Context) {
	err := fmt.Errorf("something went wrong")
	h.writer.InternalServerError(c, err)
}

func main() {

	config := server.DefaultConfig(server.ServerHTTP)
	config.Port = "8080"
	config.Environment = "dev"
	config.Version = "1.0.0"

	// Assume you already have your logger
	logger := &api.DefaultLogger{}
	handler := NewSampleHandler(logger)

	s := server.NewHTTPServer(
		config,
		server.WithLogger(logger),
		server.WithHandler(handler),
	)

	if err := s.Start(); err != nil {
		logger.Error("Server failed", err)
	}
}
