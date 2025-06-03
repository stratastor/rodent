// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package netmage

import (
	"testing"

	"github.com/stratastor/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseResolvectlStatus(t *testing.T) {
	// Create a manager instance for testing
	log, err := logger.NewTag(logger.Config{LogLevel: "debug"}, "test")
	require.NoError(t, err)

	manager := &manager{logger: log}

	t.Run("ParseValidOutput", func(t *testing.T) {
		resolvectlOutput := `Global
                 Protocols: -LLMNR -mDNS -DNSOverTLS DNSSEC=no/unsupported
          resolv.conf mode: stub
        Current DNS Server: 172.31.11.191
               DNS Servers: 172.31.11.191 1.1.1.1
                DNS Domain: ad.strata.internal

        Link 2 (enX0)
            Current Scopes: DNS
                 Protocols: +DefaultRoute -LLMNR -mDNS -DNSOverTLS DNSSEC=no/unsupported
        Current DNS Server: 172.31.11.191
               DNS Servers: 172.31.11.191 1.1.1.1
                DNS Domain: ad.strata.internal`

		dns, err := manager.parseResolvectlStatus(resolvectlOutput)
		require.NoError(t, err)

		assert.Equal(t, []string{"172.31.11.191", "1.1.1.1"}, dns.Addresses)
		assert.Equal(t, []string{"ad.strata.internal"}, dns.Search)
	})

	t.Run("ParseMultipleDomains", func(t *testing.T) {
		resolvectlOutput := `Global
                 Protocols: -LLMNR -mDNS -DNSOverTLS DNSSEC=no/unsupported
          resolv.conf mode: stub
               DNS Servers: 8.8.8.8 1.1.1.1
                DNS Domain: example.com test.local

        Link 2 (enX0)
            Current Scopes: DNS`

		dns, err := manager.parseResolvectlStatus(resolvectlOutput)
		require.NoError(t, err)

		assert.Equal(t, []string{"8.8.8.8", "1.1.1.1"}, dns.Addresses)
		assert.Equal(t, []string{"example.com", "test.local"}, dns.Search)
	})

	t.Run("ParseEmptyDNS", func(t *testing.T) {
		resolvectlOutput := `Global
                 Protocols: -LLMNR -mDNS -DNSOverTLS DNSSEC=no/unsupported
          resolv.conf mode: stub

        Link 2 (enX0)
            Current Scopes: DNS`

		dns, err := manager.parseResolvectlStatus(resolvectlOutput)
		require.NoError(t, err)

		assert.Empty(t, dns.Addresses)
		assert.Empty(t, dns.Search)
	})

	t.Run("ParseNoGlobalSection", func(t *testing.T) {
		resolvectlOutput := `Link 2 (enX0)
            Current Scopes: DNS
               DNS Servers: 192.168.1.1`

		dns, err := manager.parseResolvectlStatus(resolvectlOutput)
		require.NoError(t, err)

		// Should return empty config if no Global section
		assert.Empty(t, dns.Addresses)
		assert.Empty(t, dns.Search)
	})
}
