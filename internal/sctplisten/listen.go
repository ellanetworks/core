// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sctplisten

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/netutil"
	"github.com/ellanetworks/core/sctp"
	"go.uber.org/zap"
)

var errNoInterfaceAddrs = errors.New("no IP addresses found")

func Listen(ctx context.Context, log *zap.Logger, address string, port int, interfaceName string) (*sctp.Listener, error) {
	var laddr *sctp.SCTPAddr

	// A bind can transiently fail while a shared N2/N3 interface flaps; retry
	// resolve and listen together.
	bind := func() error {
		if interfaceName != "" {
			iface, err := net.InterfaceByName(interfaceName)
			if err != nil {
				return fmt.Errorf("failed to get interface %s: %w", interfaceName, err)
			}

			addrs, err := iface.Addrs()
			if err != nil {
				return fmt.Errorf("failed to get interface addresses: %w", err)
			}

			var ipAddrs []net.IPAddr

			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if !ok {
					continue
				}

				ip := ipNet.IP
				if ip.IsLoopback() {
					continue
				}

				if ip.IsLinkLocalUnicast() {
					continue
				}

				ipAddrs = append(ipAddrs, net.IPAddr{IP: ip})
			}

			if len(ipAddrs) == 0 {
				return fmt.Errorf("%w on interface %s", errNoInterfaceAddrs, interfaceName)
			}

			laddr = &sctp.SCTPAddr{IPAddrs: ipAddrs, Port: port}
		} else {
			netAddr, err := net.ResolveIPAddr("ip", address)
			if err != nil {
				return fmt.Errorf("error resolving address %q: %w", address, err)
			}

			laddr = &sctp.SCTPAddr{IPAddrs: []net.IPAddr{*netAddr}, Port: port}
		}

		return nil
	}

	isTransient := func(err error) bool {
		return errors.Is(err, errNoInterfaceAddrs) || netutil.IsAddrNotAvailable(err)
	}

	var listener *sctp.Listener

	var bindDevice string

	err := netutil.Retry(ctx, netutil.BindTimeout, netutil.BindInterval, isTransient, func() error {
		if err := bind(); err != nil {
			return err
		}

		device, err := netutil.VRFBindDevice(interfaceName, address)
		if err != nil {
			return err
		}

		bindDevice = device

		var lc sctp.ListenConfig
		if bindDevice != "" {
			lc.Control = netutil.BindToDeviceControl(bindDevice)
		}

		l, err := lc.Listen(ctx, laddr)
		if err != nil {
			return err
		}

		listener = l

		return nil
	})
	if err != nil {
		return nil, err
	}

	if interfaceName != "" || bindDevice != "" {
		log.Info("SCTP listener bound", zap.String("address", listener.Addr().String()),
			zap.String("interface_name", interfaceName), zap.String("vrf", bindDevice))
	}

	return listener, nil
}
