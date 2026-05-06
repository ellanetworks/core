// Copyright 2025 Ghislain Bourgeois
// Copyright 2025 Ella Networks Inc.
// SPDX-License-Identifier: GPL-3.0-or-later

package gnb

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

const (
	gtpHeaderLen int    = 16
	gtpExtLen    uint16 = 8
)

type Tunnel struct {
	Name    string
	tunIF   *water.Interface
	link    netlink.Link
	upfAddr *net.UDPAddr
	ulteid  uint32
	dlteid  uint32
	qfi     uint8
}

type NewTunnelOpts struct {
	UEIP             string
	UEIPV6           string
	UpfIP            string
	TunInterfaceName string
	ULteid           uint32
	DLteid           uint32
	MTU              uint16
	QFI              uint8
}

func (g *GnodeB) AddTunnel(opts *NewTunnelOpts) (*Tunnel, error) {
	config := water.Config{
		DeviceType: water.TUN,
	}

	config.Name = opts.TunInterfaceName

	ifce, err := water.New(config)
	if err != nil {
		return nil, fmt.Errorf("could not open TUN interface: %v", err)
	}

	eth, err := netlink.LinkByName(ifce.Name())
	if err != nil {
		return nil, fmt.Errorf("cannot read TUN interface: %v", err)
	}

	err = netlink.LinkSetUp(eth)
	if err != nil {
		return nil, fmt.Errorf("could not set TUN interface UP: %v", err)
	}

	// Give the kernel time to auto-generate the link-local address
	// before we try to remove it.
	time.Sleep(20 * time.Millisecond)

	err = netlink.LinkSetMTU(eth, int(opts.MTU))
	if err != nil {
		return nil, fmt.Errorf("could not set MTU on TUN interface: %v", err)
	}

	// Delete the kernel's auto-generated link-local address so that
	// we can add our own (derived from the IID received in the PDU
	// Session Establishment Accept). The kernel needs a link-local
	// source address to send Router Solicitations.
	for i := 0; i < 3; i++ {
		err = delAutoLinkLocal(eth)
		if err == nil {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if err != nil {
		return nil, fmt.Errorf("could not clean up auto-assigned link-local addresses: %v", err)
	}

	if opts.UEIP != "" {
		ueAddr, err := netlink.ParseAddr(opts.UEIP)
		if err != nil {
			return nil, fmt.Errorf("could not parse UE address: %v", err)
		}

		err = netlink.AddrAdd(eth, ueAddr)
		if err != nil {
			return nil, fmt.Errorf("could not assign UE address to TUN interface: %v", err)
		}
	}

	if opts.UEIPV6 != "" {
		ueAddrV6, err := netlink.ParseAddr(opts.UEIPV6)
		if err != nil {
			return nil, fmt.Errorf("could not parse UE IPv6 address: %v", err)
		}

		err = netlink.AddrAdd(eth, ueAddrV6)
		if err != nil {
			return nil, fmt.Errorf("could not assign UE IPv6 address to TUN interface: %v", err)
		}
	}

	time.Sleep(3 * time.Second)

	t := &Tunnel{
		Name:   ifce.Name(),
		tunIF:  ifce,
		link:   eth,
		ulteid: opts.ULteid,
		dlteid: opts.DLteid,
		upfAddr: &net.UDPAddr{
			IP:   net.ParseIP(opts.UpfIP),
			Port: 2152,
		},
		qfi: opts.QFI,
	}

	g.mu.Lock()
	g.tunnels[opts.DLteid] = t
	g.mu.Unlock()

	go tunToGtp(g.N3Conn, t)

	// Add a route so the kernel knows how to reach the N6 network
	// through this TUN interface. Without this, ping6 returns
	// "Network is unreachable" because the kernel has no route
	// to the N6 subnet (fd00:6::/64) in the container's routing table.
	if err := addTunRoute(eth); err != nil {
		return nil, fmt.Errorf("could not add route for N6 network via TUN interface: %v", err)
	}

	return t, nil
}

func (g *GnodeB) CloseTunnel(dlteid uint32) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	t, ok := g.tunnels[dlteid]
	if !ok {
		return fmt.Errorf("no tunnel with DL TEID %d", dlteid)
	}

	if t.link != nil {
		if err := delTunRoute(t.link); err != nil {
			logger.GnbLogger.Error("error deleting TUN route", zap.String("if", t.Name), zap.Error(err))
		}
	}

	err := t.tunIF.Close()
	if err != nil {
		logger.GnbLogger.Error("error closing TUN interface", zap.String("if", t.Name), zap.Error(err))
	}

	link, err := netlink.LinkByName(t.Name)
	if err == nil {
		if err = netlink.LinkDel(link); err != nil {
			logger.GnbLogger.Error("error deleting TUN interface", zap.String("if", t.Name), zap.Error(err))
		}
	}

	delete(g.tunnels, dlteid)

	return nil
}

func (g *GnodeB) GTPReader() { // nolint:gocognit
	buf := make([]byte, 2000)

	for {
		n, _, err := g.N3Conn.ReadFrom(buf)
		if err != nil {
			if isClosedErr(err) {
				return
			}

			logger.GnbLogger.Error("error reading from GTP-U socket", zap.Error(err))

			continue
		}

		if n < 8 {
			continue // too short
		}

		// GTPv1-U header
		if buf[0]&0x30 != 0x30 || buf[1] != 0xFF {
			continue // not a T-PDU
		}

		teid := binary.BigEndian.Uint32(buf[4:8])

		g.mu.Lock()

		t, ok := g.tunnels[teid]
		g.mu.Unlock()

		if !ok {
			logger.GnbLogger.Warn("unknown TEID, dropping packet", zap.Uint32("teid", teid))
			continue
		}

		payloadStart := 8
		if buf[0]&0x07 > 0 {
			if payloadStart+3 > n {
				logger.GnbLogger.Warn("GTP packet too short for optional fields", zap.Int("length", n))
				continue
			}

			payloadStart += 3
		}

		if buf[0]&0x04 > 0 {
			for {
				if payloadStart >= n {
					logger.GnbLogger.Warn("GTP extension header exceeds packet bounds", zap.Int("payloadStart", payloadStart), zap.Int("length", n))
					break
				}

				if buf[payloadStart] == 0x00 {
					payloadStart++
					break
				}

				if payloadStart+1 >= n {
					logger.GnbLogger.Warn("GTP extension header length byte out of bounds", zap.Int("payloadStart", payloadStart), zap.Int("length", n))
					break
				}

				extLen := int(buf[payloadStart+1]) * 4
				if extLen == 0 {
					logger.GnbLogger.Warn("GTP extension header has zero length, dropping packet")
					break
				}

				payloadStart += extLen
			}
		}

		if payloadStart > n {
			logger.GnbLogger.Warn("GTP payload start exceeds packet bounds", zap.Int("payloadStart", payloadStart), zap.Int("length", n))
			continue
		}

		_, err = t.tunIF.Write(buf[payloadStart:n])
		if err != nil {
			logger.GnbLogger.Error("error writing to TUN interface", zap.Error(err))
			continue
		}

		logger.GnbLogger.Debug("Sent packet to TUN",
			zap.String("if", t.Name),
			zap.Uint32("teid", teid),
			zap.Int("length", n-payloadStart),
		)
	}
}

func tunToGtp(conn *net.UDPConn, t *Tunnel) {
	packet := make([]byte, 2000)
	packet[0] = 0x34                                  // Version 1, Protocol type GTP, next extension header present
	packet[1] = 0xFF                                  // Message type T-PDU
	binary.BigEndian.PutUint16(packet[2:4], 0)        // Length
	binary.BigEndian.PutUint32(packet[4:8], t.ulteid) // TEID
	binary.BigEndian.PutUint32(packet[8:12], 0)       // padding
	packet[11] = 0x85                                 // ext header type: PDU Session container
	packet[12] = 0x01                                 // ext header length
	packet[13] = 0x10                                 // UL PDU Session Information
	packet[14] = t.qfi                                // QFI
	packet[15] = 0x00                                 // No more ext headers

	for {
		n, err := t.tunIF.Read(packet[gtpHeaderLen:])
		if err != nil {
			if isClosedErr(err) {
				return
			}

			logger.GnbLogger.Error("error reading from TUN interface", zap.Error(err))

			return
		}

		if n == 0 {
			logger.GnbLogger.Info("read 0 bytes")
			continue
		}

		binary.BigEndian.PutUint16(packet[2:4], uint16(n)+gtpExtLen)

		_, err = conn.WriteToUDP(packet[:n+gtpHeaderLen], t.upfAddr)
		if err != nil {
			if isClosedErr(err) {
				return
			}

			logger.GnbLogger.Error("error writing to GTP-U socket", zap.Error(err))

			continue
		}

		logger.GnbLogger.Debug(
			"Sent packet to GTP",
			zap.Int("length", n),
			zap.Int("TEID", int(t.ulteid)),
		)
	}
}

func delAutoLinkLocal(eth netlink.Link) error {
	addrs, err := netlink.AddrList(eth, netlink.FAMILY_V6)
	if err != nil {
		return fmt.Errorf("could not list IPv6 addresses: %v", err)
	}

	for _, addr := range addrs {
		if addr.IP.IsLinkLocalUnicast() {
			if err := netlink.AddrDel(eth, &addr); err != nil {
				return fmt.Errorf("could not delete link-local address %s: %v", addr.IP.String(), err)
			}

			logger.GnbLogger.Debug("Deleted link-local address", zap.String("address", addr.IP.String()))
		}
	}

	return nil
}

// addTunRoute adds an IPv6 route to the N6 network (fd00:6::/64) via the
// given TUN interface. This is needed because the container's routing table
// does not have a route to the N6 network — the only path is through the
// GTP tunnel, which requires the kernel to deliver packets to the TUN
// interface.
func addTunRoute(eth netlink.Link) error {
	_, dst, err := net.ParseCIDR("fd00:6::/64")
	if err != nil {
		return fmt.Errorf("parse N6 subnet: %w", err)
	}

	route := &netlink.Route{
		LinkIndex: eth.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       dst,
	}

	if err := netlink.RouteAdd(route); err != nil {
		// Ignore "file exists" errors — another tunnel already added it.
		if !strings.Contains(err.Error(), "exists") {
			return fmt.Errorf("add route fd00:6::/64 via %s: %w", eth.Attrs().Name, err)
		}
	}

	logger.GnbLogger.Debug("Added route for N6 network via TUN interface", zap.String("interface", eth.Attrs().Name))

	return nil
}

// delTunRoute removes the IPv6 route to the N6 network from the given TUN
// interface.
func delTunRoute(eth netlink.Link) error {
	_, dst, err := net.ParseCIDR("fd00:6::/64")
	if err != nil {
		return fmt.Errorf("parse N6 subnet: %w", err)
	}

	route := &netlink.Route{
		LinkIndex: eth.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       dst,
	}

	if err := netlink.RouteDel(route); err != nil {
		logger.GnbLogger.Debug("Could not delete route (may not exist)", zap.String("interface", eth.Attrs().Name), zap.Error(err))
	}

	return nil
}

// WaitForULAAddr waits for an IPv6 ULA address to appear on the given
// TUN interface. After the TUN interface is configured with the
// link-local address (fe80::IID), the kernel sends a Router
// Solicitation, the core responds with a Router Advertisement
// containing the delegated prefix (e.g. fd45::/64), and the kernel
// auto-configures the ULA address. This function polls until that
// address appears or the timeout expires.
func WaitForULAAddr(ifName string, prefix string, timeout time.Duration) error {
	start := time.Now()

	for time.Since(start) < timeout {
		link, err := netlink.LinkByName(ifName)
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		addrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		for _, addr := range addrs {
			if addr.IP.IsGlobalUnicast() && addr.IP.String()[:3] == prefix[:3] {
				logger.GnbLogger.Debug("ULA address appeared on TUN interface",
					zap.String("interface", ifName),
					zap.String("address", addr.IP.String()),
				)

				return nil
			}
		}

		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for ULA address on %s (prefix %s)", ifName, prefix)
}
