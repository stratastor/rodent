/*
 * Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
 * Copyright 2025 The StrataSTOR Authors and Contributors
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

package domain

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/config"
	"github.com/stratastor/rodent/internal/services/domain"
)

func NewDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain",
		Short: "Manage Active Directory domain membership",
		Long:  `Join, leave, or check status of Active Directory domain membership`,
	}

	cmd.AddCommand(newJoinCmd())
	cmd.AddCommand(newLeaveCmd())
	cmd.AddCommand(newStatusCmd())

	return cmd
}

func newJoinCmd() *cobra.Command {
	var (
		realm         string
		dcServers     []string
		adminUser     string
		adminPassword string
		waitTimeout   int
	)

	cmd := &cobra.Command{
		Use:   "join",
		Short: "Join the host to an Active Directory domain",
		Long:  `Join this host to an Active Directory domain using the specified credentials`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			// Setup logger
			cfg := config.GetConfig()
			logCfg := config.NewLoggerConfig(cfg)
			l, err := logger.NewTag(logCfg, "domain")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
				os.Exit(1)
			}

			// Create domain client
			client, err := domain.NewClient(l)
			if err != nil {
				l.Error("Failed to create domain client", "error", err)
				os.Exit(1)
			}

			// Get configuration
			var domainCfg *domain.DomainConfig
			if realm != "" {
				// Use command-line parameters
				domainCfg = &domain.DomainConfig{
					Realm:         realm,
					DCServers:     dcServers,
					AdminUser:     adminUser,
					AdminPassword: adminPassword,
				}
			} else {
				// Use global configuration
				domainCfg = domain.GetConfigFromGlobal()
			}

			// Wait for DC to be ready if specified
			if waitTimeout > 0 && len(domainCfg.DCServers) > 0 {
				l.Info("Waiting for domain controller to be ready...",
					"dc", domainCfg.DCServers[0],
					"timeout", waitTimeout)
				if err := client.WaitForDC(ctx, domainCfg.DCServers[0],
					time.Duration(waitTimeout)*time.Second); err != nil {
					l.Warn("Domain controller may not be ready", "error", err)
				} else {
					l.Info("Domain controller is ready")
				}
			}

			// Join domain
			l.Info("Joining domain", "realm", domainCfg.Realm)
			if err := client.Join(ctx, domainCfg); err != nil {
				l.Error("Failed to join domain", "error", err)
				os.Exit(1)
			}

			l.Info("Successfully joined domain", "realm", domainCfg.Realm)
			fmt.Printf("Successfully joined domain: %s\n", domainCfg.Realm)
		},
	}

	cmd.Flags().StringVar(&realm, "realm", "", "AD realm (e.g., AD.CORP.COM)")
	cmd.Flags().StringSliceVar(&dcServers, "dc", []string{}, "Domain controller servers (can be specified multiple times)")
	cmd.Flags().StringVar(&adminUser, "user", "Administrator", "Admin username for domain join")
	cmd.Flags().StringVar(&adminPassword, "password", "", "Admin password for domain join")
	cmd.Flags().IntVar(&waitTimeout, "wait", 0, "Wait for DC to be ready (seconds, 0 = no wait)")

	return cmd
}

func newLeaveCmd() *cobra.Command {
	var (
		adminUser     string
		adminPassword string
	)

	cmd := &cobra.Command{
		Use:   "leave",
		Short: "Leave the Active Directory domain",
		Long:  `Remove this host from the Active Directory domain`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			// Setup logger
			cfg := config.GetConfig()
			logCfg := config.NewLoggerConfig(cfg)
			l, err := logger.NewTag(logCfg, "domain")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
				os.Exit(1)
			}

			// Create domain client
			client, err := domain.NewClient(l)
			if err != nil {
				l.Error("Failed to create domain client", "error", err)
				os.Exit(1)
			}

			// Get configuration
			domainCfg := domain.GetConfigFromGlobal()
			if adminUser != "" {
				domainCfg.AdminUser = adminUser
			}
			if adminPassword != "" {
				domainCfg.AdminPassword = adminPassword
			}

			// Leave domain
			l.Info("Leaving domain")
			if err := client.Leave(ctx, domainCfg); err != nil {
				l.Error("Failed to leave domain", "error", err)
				os.Exit(1)
			}

			l.Info("Successfully left domain")
			fmt.Println("Successfully left domain")
		},
	}

	cmd.Flags().StringVar(&adminUser, "user", "", "Admin username (defaults to config)")
	cmd.Flags().StringVar(&adminPassword, "password", "", "Admin password (defaults to config)")

	return cmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check domain membership status",
		Long:  `Check if this host is joined to an Active Directory domain`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			// Setup logger
			cfg := config.GetConfig()
			logCfg := config.NewLoggerConfig(cfg)
			l, err := logger.NewTag(logCfg, "domain")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
				os.Exit(1)
			}

			// Create domain client
			client, err := domain.NewClient(l)
			if err != nil {
				l.Error("Failed to create domain client", "error", err)
				os.Exit(1)
			}

			// Check status
			joined, domainInfo, err := client.Status(ctx)
			if err != nil {
				l.Error("Failed to check domain status", "error", err)
				os.Exit(1)
			}

			if joined {
				fmt.Printf("Domain: %s\n", domainInfo)
				fmt.Println("Status: Joined")
			} else {
				fmt.Println("Status: Not joined to any domain")
			}
		},
	}
}
