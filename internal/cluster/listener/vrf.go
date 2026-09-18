// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package listener

import (
	"context"
	"net"
	"time"

	"github.com/ellanetworks/core/internal/netutil"
)

const resolveTimeout = 5 * time.Second

var clusterLookupIP = net.DefaultResolver.LookupIP

func vrfDeviceForBindAddress(ctx context.Context, bindAddr string) string {
	host, _, err := net.SplitHostPort(bindAddr)
	if err != nil || host == "" {
		return ""
	}

	for _, ip := range resolveBindHost(ctx, host) {
		device, err := netutil.VRFDeviceForAddress(ip.String())
		if err != nil {
			continue
		}

		if device != "" {
			return device
		}
	}

	return ""
}

func resolveBindHost(ctx context.Context, host string) []net.IP {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}
	}

	ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	ips, err := clusterLookupIP(ctx, "ip", host)
	if err != nil {
		return nil
	}

	return ips
}
