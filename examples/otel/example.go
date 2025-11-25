package main

import (
	"context"
	"log"
	"time"

	"github.com/bignyap/go-utilities/otel/api"
	"github.com/bignyap/go-utilities/otel/config"
	"github.com/bignyap/go-utilities/otel/factory"
	"github.com/bignyap/go-utilities/otel/middleware"
	"github.com/gin-gonic/gin"
)

func main() {
	// Create OpenTelemetry provider with console output for development
	cfg := config.DevelopmentConfig()
	cfg.Resource.ServiceName = "otel-example"
	cfg.Resource.ServiceVersion = "1.0.0"
	cfg.Resource.ServiceEnvironment = "development"

	provider, err := factory.NewProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to create OpenTelemetry provider: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down provider: %v", err)
		}
	}()

	// Create Gin router
	r := gin.Default()

	// Add OpenTelemetry middleware for automatic instrumentation
	r.Use(middleware.OtelMiddleware("otel-example", provider))
	r.Use(middleware.MetricsMiddleware(provider))

	// Example route with manual span creation
	r.GET("/hello/:name", func(c *gin.Context) {
		tracer := provider.Tracer("otel-example")
		ctx, span := tracer.Start(c.Request.Context(), "greet-user")
		defer span.End()

		name := c.Param("name")

		// Add attributes to span
		api.SetSpanAttributes(ctx,
			api.StringAttr("user.name", name),
			api.StringAttr("greeting.type", "hello"),
		)

		// Simulate some work
		time.Sleep(50 * time.Millisecond)

		// Add event to span
		api.AddSpanEvent(ctx, "Greeting generated",
			api.StringAttr("message", "Hello, "+name),
		)

		c.JSON(200, gin.H{
			"message": "Hello, " + name + "!",
		})
	})

	// Example route with metrics
	r.GET("/metrics-demo", func(c *gin.Context) {
		meter := provider.Meter("otel-example")

		// Create and record a counter
		counter, _ := meter.Int64Counter(
			"demo.requests",
		)
		counter.Add(c.Request.Context(), 1)

		c.JSON(200, gin.H{
			"message": "Metrics recorded",
		})
	})

	// Example route with error handling
	r.GET("/error", func(c *gin.Context) {
		ctx := c.Request.Context()

		// Simulate an error
		err := simulateError(ctx, provider)
		if err != nil {
			api.RecordError(ctx, err)
			c.JSON(500, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(200, gin.H{
			"message": "Success",
		})
	})

	log.Println("Server starting on :8080")
	log.Println("Try:")
	log.Println("  curl http://localhost:8080/hello/World")
	log.Println("  curl http://localhost:8080/metrics-demo")
	log.Println("  curl http://localhost:8080/error")

	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func simulateError(ctx context.Context, provider api.Provider) error {
	tracer := provider.Tracer("otel-example")
	ctx, span := tracer.Start(ctx, "simulate-error")
	defer span.End()

	// Simulate some processing
	time.Sleep(20 * time.Millisecond)

	// Return an error
	err := &CustomError{Message: "Something went wrong"}
	span.RecordError(err)
	return err
}

type CustomError struct {
	Message string
}

func (e *CustomError) Error() string {
	return e.Message
}
