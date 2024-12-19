// Gist of what's happening:
//
// We're using Gin's Engine (gin.New()) which provides:
// - A router with middleware support
// - HTTP handler implementation (ServeHTTP)
// - Recovery middleware for handling panics
// And then we add custom middlewares for logging, Sentry, etc.
//
// When assigned to http.Server.Handler, we're using Gin's ServeHTTP method
// since gin.Engine implements http.Handler interface
//
// This gives us several benefits:
// - Graceful Shutdown: Using http.Server gives us control over graceful shutdown through the Shutdown() method
// - Context Integration: We can properly integrate with the application's context for lifecycle management
// - Timeouts: We can set various timeouts (read, write, idle) on the server
// - Error Handling: Better control over startup errors and shutdown process
// - Middleware: Still have access to all of Gin's middleware and routing features
// - Customization: Can configure additional http.Server options like TLS, custom error handlers, etc.
//
// The main tradeoff is slightly more complex(strange?) code compared to gin.Run(), but the benefits of proper lifecycle management and graceful shutdown make it worthwhile for a production service.
// This setup integrates well with our lifecycle package for signal handling and graceful shutdown.
//

package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
)

// TODO: Review this logic
var srv *http.Server

func Start(ctx context.Context, port int) error {
	// TODO: Exclude logging source file info
	l, err := logger.NewTag(config.NewLoggerConfig(config.GetConfig()), "server")
	if err != nil {
		return err
	}
	cfg := config.GetConfig()

	// Switch to debug mode for non-production environments
	switch cfg.Environment {
	case "prod", "production":
		gin.SetMode(gin.ReleaseMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	// Create engine without middleware
	engine := gin.New()

	engine.Use(gin.Recovery())

	// Logging middleware
	engine.Use(LoggerMiddleware(l))

	// Register routes
	engine.GET("/health", func(c *gin.Context) {
		// TODO: Add sphisticated health check for Rodent
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: engine,
	}

	// Channel to catch server startup errors
	errChan := make(chan error, 1)

	// While gin.Run() would be simpler, it:
	// - Doesn't support graceful shutdown
	// - Blocks until the server exits
	// - Doesn't integrate with our context-based lifecycle management from lifecycle package
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				errChan <- err
			}
		}
	}()

	// Wait for either server error or context cancellation
	select {
	case err := <-errChan:
		return fmt.Errorf("server startup failed: %w", err)
	case <-ctx.Done():
		return Shutdown(ctx)
	}
}

func Shutdown(ctx context.Context) error {
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// LoggerMiddleware creates a dedicated middleware function for better reusability and testing
func LoggerMiddleware(l logger.Logger) gin.HandlerFunc {
	// This is to convert slog.Attr slice to a flat list of arguments
	logAttrs := func(attrs []slog.Attr) []any {
		args := make([]any, len(attrs)*2)
		for i, attr := range attrs {
			args[i*2] = attr.Key
			args[i*2+1] = attr.Value.Any()
		}
		return args
	}

	return func(c *gin.Context) {
		// Track start time
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Skip logging for health check endpoints to reduce noise
		if path == "/health" {
			return
		}

		// Get request ID or generate one if not present
		requestID := c.GetHeader("X-Request-Id")
		if requestID == "" {
			requestID = uuid.New().String()
			c.Header("X-Request-Id", requestID)
		}

		// Process request
		c.Next()

		// Convert fields to slog.Attr slice for flat structure
		attrs := []slog.Attr{
			slog.String("request_id", requestID),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("query", query),
			slog.Int("status", c.Writer.Status()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.Int("bytes_out", c.Writer.Size()),
			slog.String("ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
		}

		// Add optional fields
		if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
			attrs = append(attrs, slog.String("forwarded_for", xff))
		}
		if errs := c.Errors.ByType(gin.ErrorTypePrivate).String(); errs != "" {
			attrs = append(attrs, slog.String("error", errs))
		}

		// Log with appropriate level
		switch {
		case c.Writer.Status() >= 500:
			l.Error("Server Error", logAttrs(attrs)...)
		case c.Writer.Status() >= 400:
			l.Warn("Client Error", logAttrs(attrs)...)
		default:
			l.Info("Request", logAttrs(attrs)...)
		}
	}
}
