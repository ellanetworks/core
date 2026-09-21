// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package listener

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/ellanetworks/core/internal/netutil"
)

const resolveTimeout = 5 * time.Second

var clusterLookupIP = net.DefaultResolver.LookupIP

func vrfDeviceForBindAddress(ctx context.Context, bindAddr string) string {
	_, device, err := bindAddressAndDevice(ctx, bindAddr)
	if err != nil {
		return ""
	}

	return device
}

func ResolveBindDevice(ctx context.Context, bindAddr string) (string, error) {
	_, device, err := bindAddressAndDevice(ctx, bindAddr)

	return device, err
}

func bindAddressAndDevice(ctx context.Context, bindAddr string) (string, string, error) {
	host, port, err := net.SplitHostPort(bindAddr)
	if err != nil || host == "" {
		return bindAddr, "", nil
	}

	ips, err := resolveBindHost(ctx, host)
	if err != nil {
		return "", "", err
	}

	if len(ips) == 0 {
		return "", "", fmt.Errorf("no addresses for %s", host)
	}

	selected, device := ips[0], ""

	for _, ip := range ips {
		if d, err := netutil.VRFDeviceForAddress(ip.String()); err == nil && d != "" {
			selected, device = ip, d
			break
		}
	}

	return net.JoinHostPort(selected.String(), port), device, nil
}

func resolveBindHost(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	ips, err := clusterLookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}

	return ips, nil
}
