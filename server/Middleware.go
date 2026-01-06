package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/bignyap/go-utilities/logger/api"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Middleware struct {
	logger api.Logger
	config *Config
}

func NewMiddleware(logger api.Logger, config *Config) *Middleware {
	return &Middleware{logger: logger, config: config}
}

// sensitiveQueryParams is a list of query parameter names that should be redacted in logs
var sensitiveQueryParams = []string{
	"token",
	"api_token",
	"apitoken",
	"api_key",
	"apikey",
	"password",
	"passwd",
	"pwd",
	"secret",
	"auth",
	"authorization",
	"access_token",
	"refresh_token",
	"session",
	"session_id",
	"sessionid",
	"api-key",
	"api-token",
}

// redactSensitiveQueryParams redacts sensitive query parameters from the query string
func redactSensitiveQueryParams(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}

	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		// If parsing fails, return the original query
		return rawQuery
	}

	// Check each query parameter and redact if sensitive
	for key := range values {
		keyLower := strings.ToLower(key)
		for _, sensitive := range sensitiveQueryParams {
			if keyLower == sensitive || strings.Contains(keyLower, sensitive) {
				// Redact the value but keep the key
				values.Set(key, "[REDACTED]")
				break
			}
		}
	}

	return values.Encode()
}

func (m *Middleware) Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}

		// Store trace_id in Go's context.Context for logger to extract
		ctx := context.WithValue(c.Request.Context(), api.TraceIDKey, traceID)
		c.Request = c.Request.WithContext(ctx)

		// Redact sensitive query parameters
		redactedQuery := redactSensitiveQueryParams(c.Request.URL.RawQuery)

		reqLogger := m.logger.WithTraceID(traceID).WithComponent("api").
			AddField("method", c.Request.Method).
			AddField("path", c.Request.URL.Path).
			AddField("client_ip", c.ClientIP()).
			AddField("user_agent", c.Request.UserAgent()).
			AddField("query", redactedQuery).
			AddField("trace_id", traceID)

		c.Set("logger", reqLogger)
		c.Set("trace_id", traceID)

		c.Writer.Header().Set("X-Trace-ID", traceID)
		c.Writer.Header().Set("X-Version", m.config.Version)

		reqLogger.Info(ctx, "Incoming request")

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		reqLogger = reqLogger.
			AddField("status", status).
			AddField("latency_ms", float64(latency.Microseconds())/1000.0).
			AddField("response_size", c.Writer.Size())

		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				reqLogger.Error(ctx, "Handler error", e.Err)
			}
		}

		switch {
		case status >= 500:
			reqLogger.Error(ctx, "Request failed", nil)
		case status >= 400:
			reqLogger.Warn(ctx, "Client error")
		default:
			reqLogger.Info(ctx, "Request completed")
		}
	}
}

func (m *Middleware) CORS() gin.HandlerFunc {
	return func(c *gin.Context) {

		// Set CORS headers
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			// If no Origin header, allow all origins (for direct browser navigation)
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With, X-Trace-ID, X-Version")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func (m *Middleware) MaxBodySize(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}

func (m *Middleware) PrettyLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.config.Environment != "prod" {
			fmt.Println("\033[1;36m╔══════════════════════╗\n║  🚀  NEW REQUEST     ║\n╚══════════════════════╝\033[0m")
		}
		c.Next()
	}
}

func (m *Middleware) Profiling() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.config.Environment == "dev" && c.Query("profile") == "true" {
			runtime.SetBlockProfileRate(100)
			runtime.SetMutexProfileFraction(5)
		}
		c.Next()
	}
}

func (m *Middleware) Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger := getLoggerFromContext(c)
				if logger == nil {
					logger = m.logger
				}
				logger.Error(c.Request.Context(), "Recovered panic", fmt.Errorf("%v", err))
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

func (m *Middleware) ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			logger := getLoggerFromContext(c)
			if logger == nil {
				logger = m.logger
			}
			for _, e := range c.Errors {
				logger.Error(c.Request.Context(), "Handler error", e.Err)
			}
		}
	}
}

func (m *Middleware) Apply(router *gin.Engine) {

	fmt.Println("**************************************")
	fmt.Println("Registering Middlewares:")

	if m.config.Environment != "prod" {
		fmt.Println("\tPrettyLog")
		router.Use(m.PrettyLog())
	}
	fmt.Println("\tLogger")
	router.Use(m.Logger())

	fmt.Println("\tCORS")
	router.Use(m.CORS())

	fmt.Println("\tMaxBodySize")
	router.Use(m.MaxBodySize(m.config.MaxRequestSize))

	fmt.Println("\tRecovery")
	router.Use(m.Recovery())

	fmt.Println("\tErrorHandler")
	router.Use(m.ErrorHandler())

	if m.config.Environment == "dev" || m.config.EnableProfiling {
		fmt.Println("\tProfiling")
		router.Use(m.Profiling())
	}

	fmt.Println("**************************************")
}

func getLoggerFromContext(c *gin.Context) api.Logger {
	if logger, exists := c.Get("logger"); exists {
		if l, ok := logger.(api.Logger); ok {
			return l
		}
	}
	return nil
}

func getTraceIDFromContext(c *gin.Context) string {
	if val, exists := c.Get("trace_id"); exists {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return c.GetHeader("X-Trace-ID")
}
