package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

var (
	shutdownHooks []func()
	cancel        context.CancelFunc
)

func RegisterShutdownHook(hook func()) {
	shutdownHooks = append(shutdownHooks, hook)
}

func RegisterContextCanceller(c context.CancelFunc) {
	cancel = c
}

func HandleSignals(ctx context.Context) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	for {
		select {
		case sig := <-stop:
			switch sig {
			case syscall.SIGTERM, syscall.SIGINT:
				shutdown()
				return
			case syscall.SIGHUP:
				reload()
			}
		case <-ctx.Done():
			return
		}
	}
}

func shutdown() {
	// Cancel context first
	if cancel != nil {
		cancel()
	}
	// TODO: consider reverse order?
	// 		for i := len(shutdownHooks) - 1; i >= 0; i-- {
	// 			shutdownHooks[i]()
	// 		}
	for _, hook := range shutdownHooks {
		hook()
	}
	os.Exit(0)
}

func reload() {
	fmt.Println("Reloading configuration...")
	// TODO: Logic to reload configuration
}

func EnsureSingleInstance(pidPath string) error {
	if pidPath == "" {
		return fmt.Errorf("Invalid PID File Path")
	}

	// Check if PID file exists
	if _, err := os.Stat(pidPath); err == nil {
		// Read PID file
		pidBytes, err := os.ReadFile(pidPath)
		if err != nil {
			return fmt.Errorf("failed to read PID file: %w", err)
		}

		content := strings.TrimSpace(string(pidBytes))
		if content == "" {
			// Remove stale empty PID file
			os.Remove(pidPath)
		} else {
			pid, err := strconv.Atoi(content)
			if err != nil {
				return fmt.Errorf("invalid PID format: %w", err)
			}

			// Check if process exists
			process, err := os.FindProcess(pid)
			if err == nil {
				if err := process.Signal(syscall.Signal(0)); err == nil {
					return fmt.Errorf("another instance is already running (PID: %d)", pid)
				}
			}
			// Process not running, remove stale PID file
			os.Remove(pidPath)
		}
	}

	// Write current PID to file
	currentPid := os.Getpid()
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", currentPid)), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Register cleanup on shutdown
	RegisterShutdownHook(func() {
		os.Remove(pidPath)
	})

	return nil
}
