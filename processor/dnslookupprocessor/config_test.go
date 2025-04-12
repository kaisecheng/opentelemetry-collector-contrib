// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	createValidConfig := func() Config {
		return Config{
			Resolve: LookupConfig{
				Enabled:           true,
				Context:           resource,
				Attributes:        []string{"host.name"},
				ResolvedAttribute: "host.ip",
			},
			Reverse: LookupConfig{
				Enabled:           false,
				Context:           resource,
				Attributes:        []string{"client.ip"},
				ResolvedAttribute: "client.name",
			},
			HitCacheSize:         1000,
			HitCacheTTL:          60,
			MissCacheSize:        1000,
			MissCacheTTL:         5,
			MaxRetries:           2,
			Timeout:              0.5,
			EnableSystemResolver: true,
		}
	}

	tests := []struct {
		name             string
		mutateConfigFunc func(*Config)
		expectError      bool
		errorMsg         string
	}{
		{
			name:             "Valid default configuration",
			mutateConfigFunc: func(cfg *Config) {},
			expectError:      false,
		},
		{
			name: "No enabled lookups",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve.Enabled = false
				cfg.Reverse.Enabled = false
			},
			expectError: true,
			errorMsg:    "either forward (resolve) or reverse DNS lookup must be enabled",
		},
		{
			name: "Empty resolve attribute list",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve.Attributes = []string{}
			},
			expectError: true,
			errorMsg:    "resolve configuration: at least one attribute must be specified for DNS resolution",
		},
		{
			name: "Empty reverse attribute list",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve.Enabled = false
				cfg.Reverse.Enabled = true
				cfg.Reverse.Attributes = []string{}
			},
			expectError: true,
			errorMsg:    "reverse configuration: at least one attribute must be specified for DNS resolution",
		},
		{
			name: "Missing resolve resolved_attribute",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve.ResolvedAttribute = ""
			},
			expectError: true,
			errorMsg:    "resolve configuration: resovled_attribute must be specified for DNS resolution",
		},
		{
			name: "Missing reverse resolved_attribute",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve.Enabled = false
				cfg.Reverse.Enabled = true
				cfg.Reverse.ResolvedAttribute = ""
			},
			expectError: true,
			errorMsg:    "reverse configuration: resovled_attribute must be specified for DNS resolution",
		},
		{
			name: "Invalid resolve context",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve.Context = "invalid"
			},
			expectError: true,
			errorMsg:    "resolve configuration: context must be either 'resource' or 'record'",
		},
		{
			name: "Invalid reverse context",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve.Enabled = false
				cfg.Reverse.Enabled = true
				cfg.Reverse.Context = "invalid"
			},
			expectError: true,
			errorMsg:    "reverse configuration: context must be either 'resource' or 'record'",
		},
		{
			name: "Zero timeout",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Timeout = 0
			},
			expectError: true,
			errorMsg:    "timeout must be positive",
		},
		{
			name: "Negative timeout",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Timeout = -1.0
			},
			expectError: true,
			errorMsg:    "timeout must be positive",
		},
		{
			name: "Negative max retries",
			mutateConfigFunc: func(cfg *Config) {
				cfg.MaxRetries = -1
			},
			expectError: true,
			errorMsg:    "max_retries must be non-negative",
		},
		{
			name: "Negative hit cache size",
			mutateConfigFunc: func(cfg *Config) {
				cfg.HitCacheSize = -1
			},
			expectError: true,
			errorMsg:    "hit_cache_size must be non-negative",
		},
		{
			name: "Negative miss cache size",
			mutateConfigFunc: func(cfg *Config) {
				cfg.MissCacheSize = -1
			},
			expectError: true,
			errorMsg:    "miss_cache_size must be non-negative",
		},
		{
			name: "Zero hit cache TTL",
			mutateConfigFunc: func(cfg *Config) {
				cfg.HitCacheTTL = 0
			},
			expectError: true,
			errorMsg:    "hit_cache_ttl must be positive",
		},
		{
			name: "Negative hit cache TTL",
			mutateConfigFunc: func(cfg *Config) {
				cfg.HitCacheTTL = -1
			},
			expectError: true,
			errorMsg:    "hit_cache_ttl must be positive",
		},
		{
			name: "Zero miss cache TTL",
			mutateConfigFunc: func(cfg *Config) {
				cfg.MissCacheTTL = 0
			},
			expectError: true,
			errorMsg:    "miss_cache_ttl must be positive",
		},
		{
			name: "Negative miss cache TTL",
			mutateConfigFunc: func(cfg *Config) {
				cfg.MissCacheTTL = -1
			},
			expectError: true,
			errorMsg:    "miss_cache_ttl must be positive",
		},
		{
			name: "No resolver configured",
			mutateConfigFunc: func(cfg *Config) {
				cfg.EnableSystemResolver = false
				cfg.Hostfiles = []string{}
				cfg.Nameservers = []string{}
			},
			expectError: true,
			errorMsg:    "at least one of enable_system_resolver, hostfiles, or nameservers must be specified",
		},
		{
			name: "Only hostfiles",
			mutateConfigFunc: func(cfg *Config) {
				cfg.EnableSystemResolver = false
				cfg.Hostfiles = []string{"/etc/hosts"}
				cfg.Nameservers = []string{}
			},
			expectError: false,
		},
		{
			name: "Only nameservers",
			mutateConfigFunc: func(cfg *Config) {
				cfg.EnableSystemResolver = false
				cfg.Hostfiles = []string{}
				cfg.Nameservers = []string{"8.8.8.8"}
			},
			expectError: false,
		},
		{
			name: "Only system resolver",
			mutateConfigFunc: func(cfg *Config) {
				cfg.EnableSystemResolver = true
				cfg.Hostfiles = []string{}
				cfg.Nameservers = []string{}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createValidConfig()
			tt.mutateConfigFunc(&cfg)

			err := cfg.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
