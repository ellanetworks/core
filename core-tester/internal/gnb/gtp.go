// Copyright 2025 Ghislain Bourgeois
// Copyright 2025 Ella Networks Inc.
// SPDX-License-Identifier: GPL-3.0-or-later

package gnb

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/ellanetworks/core/core-tester/internal/logger"
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
	upfAddr *net.UDPAddr
	ulteid  uint32
	dlteid  uint32
	qfi     uint8
}

type NewTunnelOpts struct {
	UEIP             string
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

	ueAddr, err := netlink.ParseAddr(opts.UEIP)
	if err != nil {
		return nil, fmt.Errorf("could not parse UE address: %v", err)
	}

	err = netlink.AddrAdd(eth, ueAddr)
	if err != nil {
		return nil, fmt.Errorf("could not assign UE address to TUN interface: %v", err)
	}

	err = netlink.LinkSetMTU(eth, int(opts.MTU))
	if err != nil {
		return nil, fmt.Errorf("could not set MTU on TUN interface: %v", err)
	}

	err = netlink.LinkSetUp(eth)
	if err != nil {
		return nil, fmt.Errorf("could not set TUN interface UP: %v", err)
	}

	tunnel := &Tunnel{
		Name:   ifce.Name(),
		tunIF:  ifce,
		ulteid: opts.ULteid,
		dlteid: opts.DLteid,
		upfAddr: &net.UDPAddr{
			IP:   net.ParseIP(opts.UpfIP),
			Port: 2152,
		},
		qfi: opts.QFI,
	}

	g.mu.Lock()
	g.tunnels[opts.DLteid] = tunnel
	g.mu.Unlock()

	go tunToGtp(g.N3Conn, tunnel)

	return tunnel, nil
}

func (g *GnodeB) CloseTunnel(dlteid uint32) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	t, ok := g.tunnels[dlteid]
	if !ok {
		return fmt.Errorf("no tunnel with DL TEID %d", dlteid)
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

func (g *GnodeB) gtpReader() {
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
			payloadStart += 3
		}

		if buf[0]&0x04 > 0 {
			for {
				if buf[payloadStart] == 0x00 {
					payloadStart++
					break
				}

				payloadStart += int(buf[payloadStart+1]) * 4
			}
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
