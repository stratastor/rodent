package health

import (
	"fmt"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/pkg/httpclient"
)

type HealthChecker struct {
	Client *httpclient.Client
	Logger logger.Logger
}

func NewHealthChecker(cfg *config.Config) *HealthChecker {
	logConfig := config.NewLoggerConfig(cfg)
	log, err := logger.NewTag(logConfig, "health")
	if err != nil {
		panic(fmt.Sprintf("Failed to create logger: %v", err))
	}
	baseURL := fmt.Sprintf("http://localhost:%d", cfg.Server.Port)
	clientConfig := httpclient.NewClientConfig()
	clientConfig.Timeout = 5 * time.Second
	clientConfig.RetryCount = 3
	clientConfig.RetryWaitTime = 2 * time.Second
	clientConfig.BaseURL = baseURL
	client := httpclient.NewClient(clientConfig)

	return &HealthChecker{
		Client: client,
		Logger: log,
	}
}

func (hc *HealthChecker) CheckHealth() error {
	cfg := config.GetConfig()

	resp, err := hc.Client.R().
		SetPathParam("endpoint", cfg.Health.Endpoint).
		Get("{endpoint}")

	if err != nil {
		hc.Logger.Error("Health check failed: %v", err)
		return err
	}

	if resp.IsSuccess() {
		fmt.Println("Health: Healthy")
	} else {
		fmt.Printf("Health: Unhealthy (%s)\n", resp.Status())
	}

	return nil
}
