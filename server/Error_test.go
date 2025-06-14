package server_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/bignyap/go-utilities/server"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNewError(t *testing.T) {
	err := server.NewError(server.ErrorBadRequest, "bad input", errors.New("validation failed"))
	assert.Equal(t, server.ErrorBadRequest, err.Type)
	assert.Contains(t, err.Error(), "bad input")
}

func TestInternalError_ToHttpStatusCode(t *testing.T) {
	err := server.NewError(server.ErrorUnauthorized, "auth failed", nil)
	assert.Equal(t, http.StatusUnauthorized, err.ToHttpStatusCode())
}

func TestToApiError_Default(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	c.Request, _ = http.NewRequest("GET", "/", nil)

	apiErr := server.ToApiError(c, errors.New("unknown"))
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	assert.Equal(t, "Internal server error", apiErr.Message)
}
