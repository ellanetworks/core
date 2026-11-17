// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package scenarios

import (
	"encoding/binary"
	"net/netip"
)

const (
	protoICMP        = 1
	icmpHeaderLen    = 8
	icmpEchoReply    = 0
	icmpEchoRequest  = 8
	ipv4ProtocolByte = 9
)

func ICMPEchoRequest(src, dst netip.Addr, id, seq uint16, payload []byte) []byte {
	icmp := make([]byte, icmpHeaderLen, icmpHeaderLen+len(payload))
	icmp[0] = icmpEchoRequest
	binary.BigEndian.PutUint16(icmp[4:6], id)
	binary.BigEndian.PutUint16(icmp[6:8], seq)
	icmp = append(icmp, payload...)
	binary.BigEndian.PutUint16(icmp[2:4], checksum(0, icmp))

	ip := make([]byte, ipv4HeaderLen, ipv4HeaderLen+len(icmp))
	ip[0] = 0x45
	binary.BigEndian.PutUint16(ip[2:4], uint16(ipv4HeaderLen+len(icmp)))
	ip[8] = packetTTL
	ip[ipv4ProtocolByte] = protoICMP
	s, d := src.As4(), dst.As4()
	copy(ip[12:16], s[:])
	copy(ip[16:20], d[:])
	binary.BigEndian.PutUint16(ip[10:12], checksum(0, ip))

	return append(ip, icmp...)
}

func ParseICMPEchoReply(packet []byte) (id, seq uint16, payload []byte, ok bool) {
	if len(packet) < ipv4HeaderLen || packet[0]>>4 != 4 || packet[ipv4ProtocolByte] != protoICMP {
		return 0, 0, nil, false
	}

	ihl := int(packet[0]&0x0f) * 4

	total := int(binary.BigEndian.Uint16(packet[2:4]))
	if ihl < ipv4HeaderLen || total > len(packet) || total < ihl+icmpHeaderLen {
		return 0, 0, nil, false
	}

	icmp := packet[ihl:total]
	if icmp[0] != icmpEchoReply || icmp[1] != 0 {
		return 0, 0, nil, false
	}

	return binary.BigEndian.Uint16(icmp[4:6]), binary.BigEndian.Uint16(icmp[6:8]), icmp[icmpHeaderLen:], true
}
