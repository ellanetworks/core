// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package scenarios

import (
	"bytes"
	"net/netip"
	"testing"
)

func echoReplyFrom(request []byte) []byte {
	reply := bytes.Clone(request)
	copy(reply[12:16], request[16:20])
	copy(reply[16:20], request[12:16])
	reply[ipv4HeaderLen] = icmpEchoReply
	reply[ipv4HeaderLen+2], reply[ipv4HeaderLen+3] = 0, 0
	sum := checksum(0, reply[ipv4HeaderLen:])
	reply[ipv4HeaderLen+2], reply[ipv4HeaderLen+3] = byte(sum>>8), byte(sum)

	return reply
}

func TestICMPEchoRequestChecksums(t *testing.T) {
	pkt := ICMPEchoRequest(netip.MustParseAddr("10.45.0.2"), netip.MustParseAddr("10.6.0.3"), 7, 9, []byte("payload"))

	if got := checksum(0, pkt[:ipv4HeaderLen]); got != 0 {
		t.Fatalf("IPv4 header checksum does not verify: %#x", got)
	}

	if got := checksum(0, pkt[ipv4HeaderLen:]); got != 0 {
		t.Fatalf("ICMP checksum does not verify: %#x", got)
	}

	if pkt[ipv4HeaderLen] != icmpEchoRequest {
		t.Fatalf("ICMP type = %d, want echo request", pkt[ipv4HeaderLen])
	}
}

func TestParseICMPEchoReply(t *testing.T) {
	req := ICMPEchoRequest(netip.MustParseAddr("10.45.0.2"), netip.MustParseAddr("10.6.0.3"), 7, 9, []byte("payload"))

	id, seq, payload, ok := ParseICMPEchoReply(echoReplyFrom(req))
	if !ok || id != 7 || seq != 9 || string(payload) != "payload" {
		t.Fatalf("ParseICMPEchoReply = %d, %d, %q, %v", id, seq, payload, ok)
	}

	if _, _, _, ok := ParseICMPEchoReply(req); ok {
		t.Fatal("parsed an echo request as a reply")
	}

	if _, _, _, ok := ParseICMPEchoReply(req[:ipv4HeaderLen+4]); ok {
		t.Fatal("parsed a truncated packet")
	}

	if _, _, _, ok := ParseICMPEchoReply(UDPPacket(netip.MustParseAddrPort("10.6.0.3:9"), netip.MustParseAddrPort("10.45.0.2:9"), nil)); ok {
		t.Fatal("parsed a UDP packet as ICMP")
	}
}
