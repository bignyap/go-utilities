package server_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bignyap/go-utilities/logger/adapters/mock"
	"github.com/bignyap/go-utilities/server"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestResponseWriter_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	rw := server.NewResponseWriter(&mock.Mock{})

	r.GET("/success", func(c *gin.Context) {
		rw.Success(c, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("GET", "/success", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
}

func TestResponseWriter_InternalServerError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	rw := server.NewResponseWriter(&mock.Mock{})

	r.GET("/fail", func(c *gin.Context) {
		rw.InternalServerError(c, errors.New("simulated failure"))
	})

	req, _ := http.NewRequest("GET", "/fail", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), `"error":"Internal server error"`)
}
