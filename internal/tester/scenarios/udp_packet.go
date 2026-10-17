// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package scenarios

import (
	"encoding/binary"
	"net/netip"
)

const (
	ipv4HeaderLen = 20
	ipv6HeaderLen = 40
	udpHeaderLen  = 8
	protoUDP      = 17
	packetTTL     = 64
)

func UDPPacket(src, dst netip.AddrPort, payload []byte) []byte {
	udp := make([]byte, udpHeaderLen, udpHeaderLen+len(payload))
	binary.BigEndian.PutUint16(udp[0:2], src.Port())
	binary.BigEndian.PutUint16(udp[2:4], dst.Port())
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpHeaderLen+len(payload)))
	udp = append(udp, payload...)

	if src.Addr().Is4() {
		ip := make([]byte, ipv4HeaderLen, ipv4HeaderLen+len(udp))
		ip[0] = 0x45
		binary.BigEndian.PutUint16(ip[2:4], uint16(ipv4HeaderLen+len(udp)))
		ip[8] = packetTTL
		ip[9] = protoUDP
		s, d := src.Addr().As4(), dst.Addr().As4()
		copy(ip[12:16], s[:])
		copy(ip[16:20], d[:])
		binary.BigEndian.PutUint16(ip[10:12], checksum(0, ip))

		return append(ip, udp...)
	}

	s, d := src.Addr().As16(), dst.Addr().As16()

	pseudo := make([]byte, 0, 40)
	pseudo = append(pseudo, s[:]...)
	pseudo = append(pseudo, d[:]...)
	pseudo = binary.BigEndian.AppendUint32(pseudo, uint32(len(udp)))
	pseudo = append(pseudo, 0, 0, 0, protoUDP)

	sum := checksum(checksum(0, pseudo)^0xffff, udp)
	if sum == 0 {
		sum = 0xffff
	}

	binary.BigEndian.PutUint16(udp[6:8], sum)

	ip := make([]byte, ipv6HeaderLen, ipv6HeaderLen+len(udp))
	ip[0] = 0x60
	binary.BigEndian.PutUint16(ip[4:6], uint16(len(udp)))
	ip[6] = protoUDP
	ip[7] = packetTTL
	copy(ip[8:24], s[:])
	copy(ip[24:40], d[:])

	return append(ip, udp...)
}

func checksum(initial uint16, b []byte) uint16 {
	sum := uint32(initial)

	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(b[i:]))
	}

	if len(b)%2 == 1 {
		sum += uint32(b[len(b)-1]) << 8
	}

	for sum > 0xffff {
		sum = sum&0xffff + sum>>16
	}

	return ^uint16(sum)
}
