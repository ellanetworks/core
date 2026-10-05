// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/netutil"
	"github.com/ellanetworks/core/internal/sctplisten"
)

type ListenConfig struct {
	Name    string
	Address string
	Port    int
}

func (l ListenConfig) enabled() bool {
	return l.Port != 0
}

func (m *Manager) listenAddrs() ([]netip.Addr, error) {
	if m.listen.Name != "" {
		addrs, err := config.UsableInterfaceAddrs(m.listen.Name)
		if err != nil {
			return nil, fmt.Errorf("read addresses of %s: %w", m.listen.Name, err)
		}

		return addrs, nil
	}

	addr, err := netip.ParseAddr(m.listen.Address)
	if err != nil {
		return nil, fmt.Errorf("invalid Diameter listen address %q: %w", m.listen.Address, err)
	}

	if addr.IsUnspecified() {
		return nil, nil
	}

	return []netip.Addr{addr.Unmap()}, nil
}

func (m *Manager) openListeners(ctx context.Context) ([]diameter.Listener, error) {
	sl, err := sctplisten.Listen(ctx, m.logger, m.listen.Address, m.listen.Port, m.listen.Name)
	if err != nil {
		return nil, fmt.Errorf("listen on SCTP port %d: %w", m.listen.Port, err)
	}

	tl, err := listenTCP(ctx, m.listen)
	if err != nil {
		_ = sl.Close()
		return nil, fmt.Errorf("listen on TCP port %d: %w", m.listen.Port, err)
	}

	return []diameter.Listener{diameter.NewSCTPListener(sl, m.slog), diameter.NewTCPListener(tl)}, nil
}

func listenTCP(ctx context.Context, l ListenConfig) (*net.TCPListener, error) {
	addr := net.JoinHostPort(l.Address, strconv.Itoa(l.Port))

	device := l.Name
	if device == "" {
		d, err := netutil.VRFDeviceForAddress(l.Address)
		if err != nil {
			return nil, err
		}

		device = d
	}

	var ln net.Listener

	err := netutil.Retry(ctx, netutil.BindTimeout, netutil.BindInterval, netutil.IsAddrNotAvailable, func() error {
		lc := net.ListenConfig{}
		if device != "" {
			lc.Control = netutil.BindToDeviceControl(device)
		}

		var err error

		ln, err = lc.Listen(ctx, "tcp", addr)

		return err
	})
	if err != nil {
		return nil, err
	}

	tl, ok := ln.(*net.TCPListener)
	if !ok {
		_ = ln.Close()
		return nil, fmt.Errorf("unexpected listener type %T", ln)
	}

	return tl, nil
}
