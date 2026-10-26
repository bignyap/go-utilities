package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMiddleware_NilProviderSafety(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Both middlewares should safely do a no-op / pass-through when provider is nil
	r.Use(CustomSpanMiddleware(nil))
	r.Use(MetricsMiddleware(nil))

	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "pong", w.Body.String())
}

