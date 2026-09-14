// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

const (
	n6WatchBackstopInterval = time.Minute
	n6WatchRetryInterval    = time.Second
	n6WatchRetryMaxInterval = time.Minute
)

func lookupN6Addresses(n6IfName string) (netip.Addr, netip.Addr) {
	var v4, v6 netip.Addr

	if ip, err := config.GetInterfaceIPFunc(n6IfName, config.IPv4); err == nil {
		if addr, err := netip.ParseAddr(ip); err == nil {
			v4 = addr
		}
	}

	if ip, err := config.GetInterfaceIPFunc(n6IfName, config.IPv6); err == nil {
		if addr, err := netip.ParseAddr(ip); err == nil {
			v6 = addr
		}
	}

	return v4, v6
}

// watchN6Addresses keeps the BGP next-hops aligned with the N6 interface.
// onIPv4Available, when set, is called after every change that leaves the
// interface with an IPv4 address. It may be nil.
func watchN6Addresses(ctx context.Context, n6IfName string, bgpService *bgp.BGPService, onIPv4Available func(netip.Addr)) {
	backoff := n6WatchRetryInterval

	for {
		updates := make(chan netlink.AddrUpdate, 16)
		done := make(chan struct{})

		if err := netlink.AddrSubscribe(updates, done); err != nil {
			logger.EllaLog.Error("Netlink address subscription failed; retrying", zap.Error(err), zap.Duration("retry_in", backoff))
		} else {
			backoff = n6WatchRetryInterval

			if watchN6Stream(ctx, done, updates, n6IfName, bgpService, onIPv4Available) {
				return
			}

			logger.EllaLog.Warn("Netlink address subscription ended; resubscribing", zap.Duration("retry_in", backoff))
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff = min(backoff*2, n6WatchRetryMaxInterval)
	}
}

func watchN6Stream(ctx context.Context, done chan struct{}, updates chan netlink.AddrUpdate, n6IfName string, bgpService *bgp.BGPService, onIPv4Available func(netip.Addr)) bool {
	defer close(done)

	backstop := time.NewTicker(n6WatchBackstopInterval)
	defer backstop.Stop()

	drain := func() bool {
		for {
			select {
			case _, ok := <-updates:
				if !ok {
					return false
				}
			default:
				return true
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return true
		case _, ok := <-updates:
			if !ok || !drain() {
				return false
			}

			reconcileN6Addresses(n6IfName, bgpService, onIPv4Available)
		case <-backstop.C:
			reconcileN6Addresses(n6IfName, bgpService, onIPv4Available)
		}
	}
}

func reconcileN6Addresses(n6IfName string, bgpService *bgp.BGPService, onIPv4Available func(netip.Addr)) {
	curV4, curV6 := bgpService.N6Addresses()

	v4, v6 := lookupN6Addresses(n6IfName)
	if v4 == curV4 && v6 == curV6 {
		return
	}

	logN6AddressChange(n6IfName, "IPv4", curV4, v4)
	logN6AddressChange(n6IfName, "IPv6", curV6, v6)

	if err := bgpService.UpdateN6Addresses(v4, v6); err != nil {
		logger.EllaLog.Warn("Failed to apply new N6 addresses to BGP", zap.Error(err))
	}

	if v4.IsValid() && onIPv4Available != nil {
		onIPv4Available(v4)
	}
}

func logN6AddressChange(n6IfName, family string, prev, cur netip.Addr) {
	switch {
	case prev == cur:
	case !cur.IsValid():
		logger.EllaLog.Warn("N6 interface lost its address; its routes are withdrawn until an address returns",
			zap.String("interface", n6IfName), zap.String("family", family), zap.String("old", prev.String()))
	case !prev.IsValid():
		logger.EllaLog.Info("N6 interface acquired an address",
			zap.String("interface", n6IfName), zap.String("family", family), zap.String("address", cur.String()))
	default:
		logger.EllaLog.Info("N6 interface address changed",
			zap.String("interface", n6IfName), zap.String("family", family),
			zap.String("old", prev.String()), zap.String("new", cur.String()))
	}
}

// adoptRouterID gives a BGP speaker that has no router ID the N6 IPv4 address
// as its identity, and stores it so every later start uses the same value.
func adoptRouterID(ctx context.Context, dbInstance *db.Database, settings *db.BGPSettings, n6AddrV4 netip.Addr) error {
	if settings.RouterID != "" {
		return nil
	}

	if !n6AddrV4.IsValid() {
		return config.ErrNoInterfaceIP
	}

	adopted := *settings
	adopted.RouterID = n6AddrV4.String()

	if err := dbInstance.UpdateBGPSettings(ctx, &adopted); err != nil {
		return err
	}

	settings.RouterID = adopted.RouterID

	logger.EllaLog.Info("Adopted the N6 IPv4 address as the BGP router ID",
		zap.String("routerID", settings.RouterID))

	return nil
}

// adoptRouterIDWhenMissing gives BGP an identity once the N6 interface has an
// IPv4 address, for the case where it had none when the node started.
func adoptRouterIDWhenMissing(ctx context.Context, dbInstance *db.Database, n6AddrV4 netip.Addr) {
	settings, err := dbInstance.GetBGPSettings(ctx)
	if err != nil {
		logger.EllaLog.Warn("Could not read BGP settings to adopt a router ID", zap.Error(err))

		return
	}

	if !settings.Enabled || settings.RouterID != "" {
		return
	}

	if err := adoptRouterID(ctx, dbInstance, settings, n6AddrV4); err != nil {
		logger.EllaLog.Warn("Could not adopt the N6 address as the BGP router ID", zap.Error(err))
	}
}
