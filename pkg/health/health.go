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
	l, err := logger.NewTag(logConfig, "health")
	if err != nil {
		panic(fmt.Sprintf("Failed to create logger: %v", err))
	}
	baseURL := fmt.Sprintf("http://localhost:%d", cfg.Server.Port)
	clientConfig := httpclient.NewClientConfig()
	clientConfig.Timeout = 5 * time.Second
	clientConfig.RetryCount = 3
	clientConfig.RetryWaitTime = 2 * time.Second
	clientConfig.BaseURL = baseURL
	if cfg.Server.LogLevel == "debug" && (cfg.Environment == "dev" || cfg.Environment == "development") {
		clientConfig.Debug = true
		clientConfig.EnableTrace = true
	} else {
		clientConfig.Debug = false
	}
	client := httpclient.NewClient(clientConfig)

	return &HealthChecker{
		Client: client,
		Logger: l,
	}
}

// TODO: Implement a sophisticated health check convering all Rodent components
func (hc *HealthChecker) CheckHealth() (string, error) {
	cfg := config.GetConfig()

	resp, err := hc.Client.R().
		SetPathParam("endpoint", cfg.Health.Endpoint).
		Get("{endpoint}")

	if err != nil {
		return "", err
	}

	if resp.IsSuccess() {
		return resp.String(), nil
	} else {
		return "", fmt.Errorf("Unhealthy. Status: %s, Response: %s", resp.Status(), resp.String())
	}
}
