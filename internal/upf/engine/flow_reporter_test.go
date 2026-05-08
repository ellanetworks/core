// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package engine_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
)

// helper function to create an in6_addr from an IPv4 address (IPv4-mapped format)
func makeIPV4Mapped(b0, b1, b2, b3 byte) ebpf.N3N6EntrypointIn6Addr {
	var addr ebpf.N3N6EntrypointIn6Addr

	b := netip.AddrFrom4([4]byte{b0, b1, b2, b3}).As16()
	b[10] = 0xff
	b[11] = 0xff
	copy(addr.In6U.U6Addr8[:], b[:])

	return addr
}

// helper function to create an in6_addr from a 128-bit IPv6 address
func makeIPV6(b ...byte) ebpf.N3N6EntrypointIn6Addr {
	var addr ebpf.N3N6EntrypointIn6Addr
	for i := 0; i < len(b) && i < 16; i++ {
		addr.In6U.U6Addr8[i] = b[i]
	}

	return addr
}

// helper function to convert port from host byte order to network byte order
// The actual code uses u16NtoHS which converts network -> host, so we reverse it
func makePortUint16(port uint16) uint16 {
	return (port >> 8) | (port << 8)
}

func TestBuildFlowReportRequestBasic(t *testing.T) {
	flow := ebpf.N3N6EntrypointFlow{
		Imsi:  1019756139935,
		Saddr: makeIPV4Mapped(192, 168, 1, 100),
		Daddr: makeIPV4Mapped(8, 8, 8, 8),
		Sport: makePortUint16(12345),
		Dport: makePortUint16(53),
		Proto: 17, // UDP
	}

	stats := ebpf.N3N6EntrypointFlowStats{
		FirstTs: uint64(1000000000),
		LastTs:  uint64(1000300000),
		Packets: 1000,
		Bytes:   500000,
	}

	req := engine.BuildFlowReportRequest(flow, stats)

	if req.SourceIP != "192.168.1.100" {
		t.Fatalf("Expected source IP 192.168.1.100, got %s", req.SourceIP)
	}

	if req.DestinationIP != "8.8.8.8" {
		t.Fatalf("Expected destination IP 8.8.8.8, got %s", req.DestinationIP)
	}

	if req.SourcePort != 12345 {
		t.Fatalf("Expected source port 12345, got %d", req.SourcePort)
	}

	if req.DestinationPort != 53 {
		t.Fatalf("Expected destination port 53, got %d", req.DestinationPort)
	}

	if req.Protocol != 17 {
		t.Fatalf("Expected protocol 17, got %d", req.Protocol)
	}

	if req.Packets != 1000 {
		t.Fatalf("Expected packets 1000, got %d", req.Packets)
	}

	if req.Bytes != 500000 {
		t.Fatalf("Expected bytes 500000, got %d", req.Bytes)
	}
}

func TestBuildFlowReportRequestDifferentProtocols(t *testing.T) {
	testCases := []struct {
		name     string
		protocol uint8
		port     uint16
	}{
		{"TCP", 6, 443},
		{"UDP", 17, 53},
		{"ICMP", 1, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flow := ebpf.N3N6EntrypointFlow{
				Imsi:  1019756139935,
				Saddr: makeIPV4Mapped(192, 168, 1, 100),
				Daddr: makeIPV4Mapped(8, 8, 8, 8),
				Sport: makePortUint16(12345),
				Dport: makePortUint16(tc.port),
				Proto: tc.protocol,
			}

			stats := ebpf.N3N6EntrypointFlowStats{
				FirstTs: 1000000000,
				LastTs:  1000300000,
				Packets: 100,
				Bytes:   50000,
			}

			req := engine.BuildFlowReportRequest(flow, stats)

			if req.Protocol != tc.protocol {
				t.Fatalf("Expected protocol %d, got %d", tc.protocol, req.Protocol)
			}

			if req.DestinationPort != tc.port {
				t.Fatalf("Expected port %d, got %d", tc.port, req.DestinationPort)
			}
		})
	}
}

func TestBuildFlowReportRequestTimestampFormatting(t *testing.T) {
	flow := ebpf.N3N6EntrypointFlow{
		Imsi:  1019756139935,
		Saddr: makeIPV4Mapped(192, 168, 1, 100),
		Daddr: makeIPV4Mapped(8, 8, 8, 8),
		Sport: makePortUint16(12345),
		Dport: makePortUint16(53),
		Proto: 17,
	}

	stats := ebpf.N3N6EntrypointFlowStats{
		FirstTs: uint64(1000000000),
		LastTs:  uint64(1000300000),
		Packets: 100,
		Bytes:   50000,
	}

	req := engine.BuildFlowReportRequest(flow, stats)

	_, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		t.Fatalf("Invalid start time format: %s (error: %v)", req.StartTime, err)
	}

	_, err = time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		t.Fatalf("Invalid end time format: %s (error: %v)", req.EndTime, err)
	}
}

func TestBuildFlowReportRequestIPAddressConversion(t *testing.T) {
	testCases := []struct {
		name        string
		saddr       ebpf.N3N6EntrypointIn6Addr
		daddr       ebpf.N3N6EntrypointIn6Addr
		expectedSrc string
		expectedDst string
	}{
		{
			"RFC1918 private network",
			makeIPV4Mapped(192, 168, 1, 100),
			makeIPV4Mapped(192, 168, 1, 1),
			"192.168.1.100",
			"192.168.1.1",
		},
		{
			"Public DNS servers",
			makeIPV4Mapped(8, 8, 8, 8),
			makeIPV4Mapped(1, 1, 1, 1),
			"8.8.8.8",
			"1.1.1.1",
		},
		{
			"Loopback",
			makeIPV4Mapped(127, 0, 0, 1),
			makeIPV4Mapped(127, 0, 0, 1),
			"127.0.0.1",
			"127.0.0.1",
		},
		{
			"Native IPv6",
			makeIPV6(0x20, 0x01, 0x0d, 0xb8, 0xab, 0xcd, 0xef, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01),
			makeIPV6(0x20, 0x01, 0x0d, 0xb8, 0xab, 0xcd, 0xef, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01),
			"2001:db8:abcd:ef01::1",
			"2001:db8:abcd:ef02::1",
		},
		{
			"IPv6 link-local",
			makeIPV6(0xfe, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01),
			makeIPV6(0xfe, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02),
			"fe80::200:0:0:1",
			"fe80::200:0:0:2",
		},
		{
			"IPv6 all-zeros",
			makeIPV6(0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00),
			makeIPV6(0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01),
			"::",
			"::1",
		},
		{
			"IPv6 with 0xFFFF at bytes 10-11 but non-zero prefix",
			makeIPV6(0xAB, 0xCD, 0xEF, 0x01, 0x23, 0x45, 0x67, 0x89, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x01, 0x02, 0x03),
			makeIPV6(0xAB, 0xCD, 0xEF, 0x01, 0x23, 0x45, 0x67, 0x89, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x01, 0x02, 0x04),
			"abcd:ef01:2345:6789:ffff:ffff:1:203",
			"abcd:ef01:2345:6789:ffff:ffff:1:204",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flow := ebpf.N3N6EntrypointFlow{
				Imsi:  1019756139935,
				Saddr: tc.saddr,
				Daddr: tc.daddr,
				Sport: makePortUint16(12345),
				Dport: makePortUint16(53),
				Proto: 17,
			}

			stats := ebpf.N3N6EntrypointFlowStats{
				FirstTs: 1000000000,
				LastTs:  1000300000,
				Packets: 100,
				Bytes:   50000,
			}

			req := engine.BuildFlowReportRequest(flow, stats)

			if req.SourceIP != tc.expectedSrc {
				t.Fatalf("Expected source IP %s, got %s", tc.expectedSrc, req.SourceIP)
			}

			if req.DestinationIP != tc.expectedDst {
				t.Fatalf("Expected destination IP %s, got %s", tc.expectedDst, req.DestinationIP)
			}
		})
	}
}

func TestBuildFlowReportRequestImsiFormatting(t *testing.T) {
	flow := ebpf.N3N6EntrypointFlow{
		Imsi:  1019756139935,
		Saddr: makeIPV4Mapped(192, 168, 1, 100),
		Daddr: makeIPV4Mapped(8, 8, 8, 8),
		Sport: makePortUint16(12345),
		Dport: makePortUint16(53),
		Proto: 17,
	}

	stats := ebpf.N3N6EntrypointFlowStats{
		FirstTs: 1000000000,
		LastTs:  1000300000,
		Packets: 100,
		Bytes:   50000,
	}

	req := engine.BuildFlowReportRequest(flow, stats)

	if req.IMSI != "001019756139935" {
		t.Fatalf("Expected IMSI 001019756139935, got %s", req.IMSI)
	}
}

func TestBuildFlowReportRequestIPv6Addresses(t *testing.T) {
	// Test that IPv6 addresses are correctly converted without the IPv4-mapped prefix
	flow := ebpf.N3N6EntrypointFlow{
		Imsi:  1019756139935,
		Saddr: makeIPV6(0x20, 0x01, 0x0d, 0xb8, 0xab, 0xcd, 0xef, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01),
		Daddr: makeIPV6(0x20, 0x01, 0x0d, 0xb8, 0xab, 0xcd, 0xef, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01),
		Sport: makePortUint16(54321),
		Dport: makePortUint16(443),
		Proto: 6, // TCP
	}

	stats := ebpf.N3N6EntrypointFlowStats{
		FirstTs: 1000000000,
		LastTs:  1000500000,
		Packets: 500,
		Bytes:   250000,
	}

	req := engine.BuildFlowReportRequest(flow, stats)

	if req.SourceIP != "2001:db8:abcd:ef01::1" {
		t.Fatalf("Expected source IP 2001:db8:abcd:ef01::1, got %s", req.SourceIP)
	}

	if req.DestinationIP != "2001:db8:abcd:ef02::1" {
		t.Fatalf("Expected destination IP 2001:db8:abcd:ef02::1, got %s", req.DestinationIP)
	}

	if req.SourcePort != 54321 {
		t.Fatalf("Expected source port 54321, got %d", req.SourcePort)
	}

	if req.DestinationPort != 443 {
		t.Fatalf("Expected destination port 443, got %d", req.DestinationPort)
	}

	if req.Protocol != 6 {
		t.Fatalf("Expected protocol 6 (TCP), got %d", req.Protocol)
	}
}

func TestBuildFlowReportRequestMixedIPv4IPv6(t *testing.T) {
	// Test that IPv4 addresses are correctly identified as IPv4-mapped
	flow := ebpf.N3N6EntrypointFlow{
		Imsi:  1019756139935,
		Saddr: makeIPV4Mapped(10, 0, 0, 1),
		Daddr: makeIPV4Mapped(172, 16, 0, 1),
		Sport: makePortUint16(8080),
		Dport: makePortUint16(80),
		Proto: 6, // TCP
	}

	stats := ebpf.N3N6EntrypointFlowStats{
		FirstTs: 2000000000,
		LastTs:  2000100000,
		Packets: 200,
		Bytes:   100000,
	}

	req := engine.BuildFlowReportRequest(flow, stats)

	if req.SourceIP != "10.0.0.1" {
		t.Fatalf("Expected source IP 10.0.0.1, got %s", req.SourceIP)
	}

	if req.DestinationIP != "172.16.0.1" {
		t.Fatalf("Expected destination IP 172.16.0.1, got %s", req.DestinationIP)
	}

	// Verify the address is treated as IPv4 (no brackets in string representation)
	if netip.MustParseAddr(req.SourceIP).Is6() {
		t.Fatal("Expected IPv4 address, got IPv6")
	}

	if !netip.MustParseAddr(req.DestinationIP).Is4() {
		t.Fatal("Expected IPv4 address, got IPv6")
	}
}
