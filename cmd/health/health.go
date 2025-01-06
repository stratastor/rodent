/*
 * Copyright 2024 Raamsri Kumar <raam@tinkershack.in> and The StrataSTOR Authors 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */package health

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/pkg/health"
)

func NewHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check Rodent health",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.GetConfig() // cfg shoudln't be nil
			checker := health.NewHealthChecker(cfg)
			ret, err := checker.CheckHealth()
			if err != nil {
				fmt.Println("Health check failed: ", err)
				return nil
			}
			fmt.Println(ret)
			return nil
		},
	}
}
