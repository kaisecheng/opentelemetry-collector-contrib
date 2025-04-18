// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/resolver"
)

var errUnknownContextID = errors.New("unknown attribute context")

type dnsLookupProcessor struct {
	config       *Config
	resolver     resolver.Resolver
	processPairs []ProcessPair
	logger       *zap.Logger
}

type ProcessPair struct {
	ContextID ContextID
	ProcessFn func(ctx context.Context, pMap pcommon.Map) error
}

func newDNSLookupProcessor(config *Config, logger *zap.Logger) (*dnsLookupProcessor, error) {
	dnsResolver, err := createResolverChain(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create resolver chain: %w", err)
	}

	dp := &dnsLookupProcessor{
		logger:   logger,
		config:   config,
		resolver: dnsResolver,
	}

	dp.processPairs = dp.createProcessPairs()

	return dp, nil
}

// createResolverChain creates a chain of resolvers based on the provided configuration.
// The resolution order is cache -> chain( hostfile -> nameserver -> system resolver ).
// Returns either a chain resolver or a cache resolver if cache is enabled.
// Returns an error if no resolvers are configured or if any of the resolvers fail to initialize.
func createResolverChain(config *Config, logger *zap.Logger) (resolver.Resolver, error) {
	var chainResolver resolver.Resolver
	var resolvers []resolver.Resolver

	if len(config.Hostfiles) > 0 {
		hostfileResolver, err := resolver.NewHostFileResolver(
			config.Hostfiles,
			logger,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create hostfile resolver: %w", err)
		}

		resolvers = append(resolvers, hostfileResolver)
	}

	if len(config.Nameservers) > 0 {
		nameserverResolver, err := resolver.NewNameserverResolver(
			config.Nameservers,
			time.Duration(config.Timeout*float64(time.Second)),
			logger,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create nameserver resolver: %w", err)
		}

		resolvers = append(resolvers, nameserverResolver)
	}

	if config.EnableSystemResolver {
		systemResolver := resolver.NewSystemResolver(
			time.Duration(config.Timeout*float64(time.Second)),
			logger,
		)
		resolvers = append(resolvers, systemResolver)
	}

	if len(resolvers) == 0 {
		return nil, fmt.Errorf("no DNS resolver configuration available: either hostfile, nameserver, or system resolver must be enabled")
	}

	chainResolver = resolver.NewChainResolver(config.MaxRetries, resolvers, logger)

	if config.HitCacheSize > 0 || config.MissCacheSize > 0 {
		cacheResolver, err := resolver.NewCacheResolver(
			chainResolver,
			config.HitCacheSize,
			time.Duration(config.HitCacheTTL)*time.Second,
			config.MissCacheSize,
			time.Duration(config.MissCacheTTL)*time.Second,
			logger,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create cache resolver: %w", err)
		}

		return cacheResolver, nil
	}

	return chainResolver, nil
}

func (dp *dnsLookupProcessor) createProcessPairs() []ProcessPair {
	if dp.config.Resolve.Enabled && dp.config.Reverse.Enabled &&
		(dp.config.Resolve.Context == dp.config.Reverse.Context) {
		return []ProcessPair{
			{
				ContextID: dp.config.Resolve.Context,
				ProcessFn: dp.processDNSLookup,
			},
		}
	}

	var processPairs []ProcessPair

	if dp.config.Resolve.Enabled {
		processPairs = append(processPairs, ProcessPair{
			ContextID: dp.config.Resolve.Context,
			ProcessFn: dp.processResolveLookup,
		})
	}

	if dp.config.Reverse.Enabled {
		processPairs = append(processPairs, ProcessPair{
			ContextID: dp.config.Reverse.Context,
			ProcessFn: dp.processReverseLookup,
		})
	}

	return processPairs
}

// processDNSLookup performs DNS lookups on a set of attributes
func (dp *dnsLookupProcessor) processDNSLookup(ctx context.Context, pMap pcommon.Map) error {
	resolveErr := dp.processResolveLookup(ctx, pMap)
	reverseErr := dp.processReverseLookup(ctx, pMap)

	return errors.Join(resolveErr, reverseErr)
}

// processResolveLookup finds the hostname from attributes and resolves it to an IP address
func (dp *dnsLookupProcessor) processResolveLookup(ctx context.Context, pMap pcommon.Map) error {
	hostname, err := targetStrFromAttributes(dp.config.Resolve.Attributes, pMap, resolver.ParseHostname)
	if err != nil {
		dp.logger.Debug("Failed to find hostname from attributes", zap.Error(err))
		if errors.Is(err, resolver.ErrInvalidHostname) {
			return nil
		}
		return err
	}

	// Found a hostname. Try to resolve it
	ip, err := dp.resolver.Resolve(ctx, hostname)
	if err == nil {
		// Success resolution. Save the result to attribute
		if len(ip) > 0 {
			pMap.PutStr(dp.config.Resolve.ResolvedAttribute, ip)
		}
		return nil
	} else if errors.Is(err, resolver.ErrNoResolution) {
		return nil
	} else {
		return err
	}
}

// processReverseLookup finds the IP from attributes and resolves it to a hostname
func (dp *dnsLookupProcessor) processReverseLookup(ctx context.Context, pMap pcommon.Map) error {
	ip, err := targetStrFromAttributes(dp.config.Reverse.Attributes, pMap, resolver.ParseIP)
	if err != nil {
		dp.logger.Debug("Failed to find IP from attributes", zap.Error(err))
		if errors.Is(err, resolver.ErrInvalidIP) {
			return nil
		}
		return err
	}

	// Found an IP. Try to resolve it
	hostname, err := dp.resolver.Reverse(ctx, ip)
	if err == nil {
		// Success resolution. Save the result to attribute
		if len(hostname) > 0 {
			pMap.PutStr(dp.config.Reverse.ResolvedAttribute, hostname)
		}
		return nil
	} else if errors.Is(err, resolver.ErrNoResolution) {
		return nil
	} else {
		return err
	}
}

// targetStrFromAttributes returns the first IP/hostname from the given attributes.
// It uses the provided validation function to check if the value is valid.
func targetStrFromAttributes(attributes []string, pMap pcommon.Map, validateFn func(string) (string, error)) (string, error) {
	var lastErr error

	for _, attr := range attributes {
		if val, found := pMap.Get(attr); found {
			if validStr, err := validateFn(val.Str()); err != nil {
				lastErr = err
			} else {
				return validStr, nil
			}
		}
	}

	return "", lastErr
}
