// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/upf/core"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

// Helper function to convert IP address to uint32 format used by eBPF
// The actual implementation uses binary.NativeEndian.PutUint32(), so we reverse it
func makeIPUint32(b0, b1, b2, b3 byte) uint32 {
	// Create a net.IP from the bytes
	ip := net.IPv4(b0, b1, b2, b3)
	// Convert to uint32 using NativeEndian (matching the actual int2ip function behavior)
	return binary.NativeEndian.Uint32(ip.To4())
}

// Helper function to convert port from host byte order to network byte order
// The actual code uses u16NtoHS which converts network -> host, so we reverse it
func makePortUint16(port uint16) uint16 {
	b := make([]byte, 2)
	binary.NativeEndian.PutUint16(b, port)

	return binary.BigEndian.Uint16(b)
}

func TestBuildFlowReportRequestBasic(t *testing.T) {
	flow := ebpf.N3N6EntrypointFlow{
		Imsi:  1019756139935,
		Saddr: makeIPUint32(192, 168, 1, 100),
		Daddr: makeIPUint32(8, 8, 8, 8),
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

	req := core.BuildFlowReportRequest(flow, stats)

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
				Saddr: makeIPUint32(192, 168, 1, 100),
				Daddr: makeIPUint32(8, 8, 8, 8),
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

			req := core.BuildFlowReportRequest(flow, stats)

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
		Saddr: makeIPUint32(192, 168, 1, 100),
		Daddr: makeIPUint32(8, 8, 8, 8),
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

	req := core.BuildFlowReportRequest(flow, stats)

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
		saddr       uint32
		daddr       uint32
		expectedSrc string
		expectedDst string
	}{
		{
			"RFC1918 private network",
			makeIPUint32(192, 168, 1, 100),
			makeIPUint32(192, 168, 1, 1),
			"192.168.1.100",
			"192.168.1.1",
		},
		{
			"Public DNS servers",
			makeIPUint32(8, 8, 8, 8),
			makeIPUint32(1, 1, 1, 1),
			"8.8.8.8",
			"1.1.1.1",
		},
		{
			"Loopback",
			makeIPUint32(127, 0, 0, 1),
			makeIPUint32(127, 0, 0, 1),
			"127.0.0.1",
			"127.0.0.1",
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

			req := core.BuildFlowReportRequest(flow, stats)

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
		Saddr: makeIPUint32(192, 168, 1, 100),
		Daddr: makeIPUint32(8, 8, 8, 8),
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

	req := core.BuildFlowReportRequest(flow, stats)

	if req.IMSI != "001019756139935" {
		t.Fatalf("Expected IMSI 001019756139935, got %s", req.IMSI)
	}
}
