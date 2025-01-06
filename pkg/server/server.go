/*
 * Copyright 2024-2025 Raamsri Kumar <raam@tinkershack.in>
 * Copyright 2024-2025 The StrataSTOR Authors and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
	"net/http"

	"github.com/gin-gonic/gin"
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

	registerZFSRoutes(engine)

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
