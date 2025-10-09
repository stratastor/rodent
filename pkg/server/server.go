// Copyright 2024 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

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
	"os"

	"github.com/gin-gonic/gin"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/events"
	"github.com/stratastor/rodent/internal/toggle"
	eventspb "github.com/stratastor/toggle-rodent-proto/proto/events"
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

	toggle.StartRegistrationProcess(ctx, l)

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

	// Register service routes
	serviceHandler, err := registerServiceRoutes(engine)
	if err != nil {
		return fmt.Errorf("failed to register service routes: %w", err)
	}
	defer serviceHandler.Close()

	err = registerZFSRoutes(engine)
	if err != nil {
		l.Error("Failed to register ZFS routes, continuing without ZFS functionality", "error", err)
	}

	// Register ACL routes with graceful error handling
	aclHandler, err := registerFaclRoutes(engine)
	if err != nil {
		l.Error("Failed to register ACL routes, continuing without ACL functionality", "error", err)
	} else {
		defer aclHandler.Close()
	}

	// Register shares routes with graceful error handling
	err = registerSharesRoutes(engine)
	if err != nil {
		l.Error(
			"Failed to register shares routes, continuing without shares functionality",
			"error",
			err,
		)
	}

	// Register SSH key routes with graceful error handling
	sshKeyHandler, err := registerSSHKeyRoutes(engine)
	if err != nil {
		l.Error(
			"Failed to register SSH key routes, continuing without SSH key functionality",
			"error",
			err,
		)
	} else {
		defer sshKeyHandler.Close()
	}

	// Register network management routes with graceful error handling
	networkHandler, err := registerNetworkRoutes(engine)
	if err != nil {
		l.Error(
			"Failed to register network routes, continuing without network management functionality",
			"error",
			err,
		)
	} else {
		_ = networkHandler // Handler doesn't implement Close() method
	}

	// Register system management routes with graceful error handling
	systemHandler, err := registerSystemRoutes(engine)
	if err != nil {
		l.Error(
			"Failed to register system routes, continuing without system management functionality",
			"error",
			err,
		)
	} else {
		_ = systemHandler // Handler doesn't implement Close() method
	}

	// Register disk management routes with graceful error handling
	diskHandler, err := registerDiskRoutes(engine)
	if err != nil {
		l.Error(
			"Failed to register disk routes, continuing without disk management functionality",
			"error",
			err,
		)
	} else {
		_ = diskHandler // Handler doesn't implement Close() method yet
	}

	// Start AD DC service if enabled in config
	if cfg.AD.DC.Enabled {
		l.Info("AD DC service is enabled, starting the service...")

		// Get the service manager
		svcManager, ok := serviceHandler.GetServiceManager()
		if !ok {
			l.Warn("Service manager not available, AD DC service will not be started")
		} else {
			// Start AD DC service
			if err := svcManager.StartService(ctx, "addc"); err != nil {
				l.Warn("Failed to start AD DC service, continuing anyway", "error", err)
			} else {
				l.Info("AD DC service started successfully")
			}
		}

		// Wait a moment for AD DC to initialize if it was just started
		l.Info("Waiting for AD DC service to initialize before registering AD routes...")
		// We don't need to sleep here as the AD client will retry connection if needed

		// Register AD routes with graceful error handling
		adHandler, err := registerADRoutes(engine)
		if err != nil {
			l.Error(
				"Failed to register AD routes, continuing without AD functionality",
				"error",
				err,
			)
		} else {
			defer adHandler.Close()
		}

	}

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

	// Emit server shutdown event with structured payload
	servicePayload := &eventspb.ServiceStatusPayload{
		ServiceName: "rodent-controller",
		Status:      "stopping",
		Pid:         int32(os.Getpid()),
		Operation:   eventspb.ServiceStatusPayload_SERVICE_STATUS_OPERATION_STOPPED,
	}

	serviceMeta := map[string]string{
		"component": "service",
		"action":    "shutdown",
		"service":   "rodent-controller",
	}

	events.EmitServiceStatus(
		eventspb.EventLevel_EVENT_LEVEL_INFO,
		servicePayload,
		serviceMeta,
	)

	return srv.Shutdown(ctx)
}
