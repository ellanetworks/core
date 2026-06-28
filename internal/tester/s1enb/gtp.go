// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

const (
	gtpUDPPort = 2152
	// gtpHeaderLen is the GTPv1-U header length for a plain G-PDU with no optional
	// sequence/N-PDU/extension fields.
	gtpHeaderLen = 8
)

// tunnel is a UE bearer's GTP-U datapath: a TUN interface bridged to the S1-U
// socket. Uplink IP packets are encapsulated to the UPF with the uplink TEID;
// downlink G-PDUs for the eNB's downlink TEID are decapsulated to the TUN.
type tunnel struct {
	name    string
	tunIF   *water.Interface
	upfAddr *net.UDPAddr
	ulteid  uint32
	dlteid  uint32
}

// TunnelOpts configures a GTP-U datapath for an attached UE's default bearer.
type TunnelOpts struct {
	UEIPv4           string // CIDR form, e.g. "10.45.0.1/16"
	UEIPv6           string // CIDR form of the link-local from the PDN IID, e.g. "fe80::.../64"
	UpfAddress       string // S-GW/UPF S1-U address (uplink target)
	ULTEID           uint32 // uplink TEID, sent to the UPF
	DLTEID           uint32 // eNB downlink TEID, for demultiplexing inbound G-PDUs
	TunInterfaceName string
	MTU              int // 0 selects a default that leaves room for GTP-U overhead
}

// AddTunnel brings up a TUN interface for the UE's bearer and forwards between it
// and the S1-U socket. With UEIPv6 set it also prepares the interface for SLAAC:
// the kernel's auto link-local is replaced with the one derived from the PDN IID,
// so the UPF's Router Advertisement yields a global address. Requires
// EnableDatapath at Start.
func (e *ENB) AddTunnel(opts *TunnelOpts) error {
	if e.n3Conn == nil {
		return fmt.Errorf("s1enb: no S1-U socket (Start with EnableDatapath)")
	}

	cfg := water.Config{DeviceType: water.TUN}
	cfg.Name = opts.TunInterfaceName

	ifce, err := water.New(cfg)
	if err != nil {
		return fmt.Errorf("s1enb: open TUN: %w", err)
	}

	link, err := netlink.LinkByName(ifce.Name())
	if err != nil {
		return fmt.Errorf("s1enb: read TUN: %w", err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("s1enb: set TUN up: %w", err)
	}

	mtu := opts.MTU
	if mtu == 0 {
		mtu = 1400
	}

	if err := netlink.LinkSetMTU(link, mtu); err != nil {
		return fmt.Errorf("s1enb: set TUN MTU: %w", err)
	}

	if opts.UEIPv4 != "" {
		addr, err := netlink.ParseAddr(opts.UEIPv4)
		if err != nil {
			return fmt.Errorf("s1enb: parse UE address %q: %w", opts.UEIPv4, err)
		}

		if err := netlink.AddrAdd(link, addr); err != nil {
			return fmt.Errorf("s1enb: assign UE address: %w", err)
		}
	}

	if opts.UEIPv6 != "" {
		if err := e.assignUEIPv6(link, opts.UEIPv6); err != nil {
			return err
		}
	}

	t := &tunnel{
		name:    ifce.Name(),
		tunIF:   ifce,
		upfAddr: &net.UDPAddr{IP: net.ParseIP(opts.UpfAddress), Port: gtpUDPPort},
		ulteid:  opts.ULTEID,
		dlteid:  opts.DLTEID,
	}

	e.mu.Lock()
	e.tunnels[opts.DLTEID] = t
	e.mu.Unlock()

	go e.tunToGTP(t)

	return nil
}

// assignUEIPv6 replaces the kernel's auto-generated link-local with the one
// derived from the PDN IID, so the UE can solicit a Router Advertisement and
// form a global address via SLAAC (TS 23.401).
func (e *ENB) assignUEIPv6(link netlink.Link, cidr string) error {
	// The kernel needs a link-local source to send Router Solicitations; remove
	// its auto-generated one so only the IID-derived link-local remains.
	var err error
	for i := 0; i < 3; i++ {
		if err = delAutoLinkLocal(link); err == nil {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if err != nil {
		return fmt.Errorf("s1enb: clean up auto link-local: %w", err)
	}

	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("s1enb: parse UE IPv6 %q: %w", cidr, err)
	}

	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("s1enb: assign UE IPv6: %w", err)
	}

	return nil
}

func delAutoLinkLocal(link netlink.Link) error {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		return fmt.Errorf("list IPv6 addresses: %w", err)
	}

	for _, addr := range addrs {
		if addr.IP.IsLinkLocalUnicast() {
			if err := netlink.AddrDel(link, &addr); err != nil {
				return fmt.Errorf("delete link-local %s: %w", addr.IP, err)
			}
		}
	}

	return nil
}

// WaitForULAAddr blocks until a non-tentative global IPv6 address in prefix has
// been formed on the interface via SLAAC, or the timeout elapses.
func WaitForULAAddr(ifName, prefix string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
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
			if !addr.IP.IsGlobalUnicast() || addr.IP.String()[:3] != prefix[:3] {
				continue
			}

			// A tentative address (still in DAD) cannot be used as a source.
			if addr.Flags&syscall.IFA_F_TENTATIVE != 0 {
				continue
			}

			return nil
		}

		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("s1enb: timeout waiting for a global IPv6 address (prefix %s) on %s", prefix, ifName)
}

// CloseTunnel tears down the tunnel for the given downlink TEID.
func (e *ENB) CloseTunnel(dlteid uint32) {
	e.mu.Lock()
	t := e.tunnels[dlteid]
	delete(e.tunnels, dlteid)
	e.mu.Unlock()

	if t != nil {
		t.close()
	}
}

func (t *tunnel) close() {
	if t.tunIF != nil {
		_ = t.tunIF.Close()
	}

	if link, err := netlink.LinkByName(t.name); err == nil {
		_ = netlink.LinkDel(link)
	}
}

func (e *ENB) tunToGTP(t *tunnel) {
	pkt := make([]byte, 2000)
	pkt[0] = 0x30 // version 1, protocol type GTP, no optional fields
	pkt[1] = 0xff // G-PDU
	binary.BigEndian.PutUint32(pkt[4:8], t.ulteid)

	for {
		n, err := t.tunIF.Read(pkt[gtpHeaderLen:])
		if err != nil {
			return
		}

		if n == 0 {
			continue
		}

		binary.BigEndian.PutUint16(pkt[2:4], uint16(n))

		if _, err := e.n3Conn.WriteToUDP(pkt[:gtpHeaderLen+n], t.upfAddr); err != nil {
			return
		}
	}
}

func (e *ENB) gtpReader() {
	buf := make([]byte, 2000)

	for {
		n, _, err := e.n3Conn.ReadFrom(buf)
		if err != nil {
			return
		}

		if n < gtpHeaderLen || buf[0]&0x30 != 0x30 || buf[1] != 0xff {
			continue
		}

		teid := binary.BigEndian.Uint32(buf[4:8])

		e.mu.Lock()
		t := e.tunnels[teid]
		e.mu.Unlock()

		if t == nil {
			continue
		}

		start := gtpHeaderLen

		// Optional sequence-number / N-PDU / extension-flag fields add 3 octets.
		if buf[0]&0x07 != 0 {
			if start+3 > n {
				continue
			}

			start += 3
		}

		// Walk any extension headers (a real UPF sends plain G-PDUs to a 4G eNB,
		// so this is defensive).
		if buf[0]&0x04 != 0 {
			for start < n {
				if buf[start] == 0x00 {
					start++
					break
				}

				if start+1 >= n {
					break
				}

				extLen := int(buf[start+1]) * 4
				if extLen == 0 {
					break
				}

				start += extLen
			}
		}

		if start > n {
			continue
		}

		if _, err := t.tunIF.Write(buf[start:n]); err != nil {
			logger.GnbLogger.Error("s1enb: write to TUN failed", zap.Error(err))
		}
	}
}
