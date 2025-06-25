package server

import (
	"net/http"

	"github.com/bignyap/go-utilities/logger/api"
	"github.com/gin-gonic/gin"
)

type ResponseWriter struct {
	logger api.Logger
}

func NewResponseWriter(logger api.Logger) *ResponseWriter {
	return &ResponseWriter{logger: logger}
}

func (rw *ResponseWriter) Success(c *gin.Context, data interface{}) {
	// c.JSON(http.StatusOK, Response{Data: data})
	c.JSON(http.StatusOK, data)
}

func (rw *ResponseWriter) Created(c *gin.Context, data interface{}) {
	// c.JSON(http.StatusCreated, Response{Data: data})
	c.JSON(http.StatusCreated, data)
}

func (rw *ResponseWriter) NoContent(c *gin.Context) {
	c.AbortWithStatus(http.StatusNoContent)
}

func (rw *ResponseWriter) Error(c *gin.Context, err error) {
	apiErr := ToApiError(c, err)

	logger := getLoggerFromContext(c)
	if logger == nil {
		logger = rw.logger
	}

	logger.WithFields(
		api.Int("code", apiErr.Code),
		api.String("message", apiErr.Message),
		api.String("trace_id", apiErr.TraceID),
	).Error("API error response", err)

	c.JSON(apiErr.Code, ErrorResponse{Error: apiErr.Message})
}

// Shorthand helpers
func (rw *ResponseWriter) BadRequest(c *gin.Context, msg string) {
	rw.Error(c, NewError(ErrorBadRequest, msg, nil))
}

func (rw *ResponseWriter) Unauthorized(c *gin.Context) {
	rw.Error(c, NewError(ErrorUnauthorized, "Unauthorized", nil))
}

func (rw *ResponseWriter) NotFound(c *gin.Context) {
	rw.Error(c, NewError(ErrorNotFound, "Not found", nil))
}

func (rw *ResponseWriter) InternalServerError(c *gin.Context, err error) {
	rw.Error(c, NewError(ErrorInternal, "Internal server error", err))
}

// Response JSON structures
type Response struct {
	Data interface{} `json:"data"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
