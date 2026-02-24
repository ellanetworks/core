// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	"github.com/ellanetworks/core/internal/upf/core"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/wmnsk/go-pfcp/message"
)

// MockSMF implements the SMF interface for testing
type MockSMF struct {
	LastFlowReport *pfcp_dispatcher.FlowReportRequest
	CallCount      int
	ShouldError    bool
	ErrorMsg       string
}

func (m *MockSMF) SendFlowReport(ctx context.Context, req *pfcp_dispatcher.FlowReportRequest) error {
	m.CallCount++

	m.LastFlowReport = req
	if m.ShouldError {
		return nil // Error would be logged, not propagated
	}

	return nil
}

func (m *MockSMF) HandlePfcpSessionReportRequest(ctx context.Context, msg *message.SessionReportRequest) (*message.SessionReportResponse, error) {
	return nil, nil
}

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

func TestSendFlowReportBasic(t *testing.T) {
	// Set up mock SMF
	mockSMF := &MockSMF{}
	originalDispatcher := pfcp_dispatcher.Dispatcher
	pfcp_dispatcher.Dispatcher = pfcp_dispatcher.PfcpDispatcher{SMF: mockSMF}

	defer func() {
		pfcp_dispatcher.Dispatcher = originalDispatcher
	}()

	// Create test flow (192.168.1.100 and 8.8.8.8)
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

	core.SendFlowReport(t.Context(), flow, stats)

	// Verify the report was sent
	if mockSMF.CallCount != 1 {
		t.Fatalf("Expected SendFlowReport to be called once, got %d calls", mockSMF.CallCount)
	}

	if mockSMF.LastFlowReport == nil {
		t.Fatalf("Expected flow report to be set, got nil")
	}

	// Verify source IP
	if mockSMF.LastFlowReport.SourceIP != "192.168.1.100" {
		t.Fatalf("Expected source IP 192.168.1.100, got %s", mockSMF.LastFlowReport.SourceIP)
	}

	// Verify destination IP
	if mockSMF.LastFlowReport.DestinationIP != "8.8.8.8" {
		t.Fatalf("Expected destination IP 8.8.8.8, got %s", mockSMF.LastFlowReport.DestinationIP)
	}

	// Verify ports
	if mockSMF.LastFlowReport.SourcePort != 12345 {
		t.Fatalf("Expected source port 12345, got %d", mockSMF.LastFlowReport.SourcePort)
	}

	if mockSMF.LastFlowReport.DestinationPort != 53 {
		t.Fatalf("Expected destination port 53, got %d", mockSMF.LastFlowReport.DestinationPort)
	}

	// Verify protocol
	if mockSMF.LastFlowReport.Protocol != 17 {
		t.Fatalf("Expected protocol 17, got %d", mockSMF.LastFlowReport.Protocol)
	}

	// Verify traffic metrics
	if mockSMF.LastFlowReport.Packets != 1000 {
		t.Fatalf("Expected packets 1000, got %d", mockSMF.LastFlowReport.Packets)
	}

	if mockSMF.LastFlowReport.Bytes != 500000 {
		t.Fatalf("Expected bytes 500000, got %d", mockSMF.LastFlowReport.Bytes)
	}
}

func TestSendFlowReportMultipleCalls(t *testing.T) {
	mockSMF := &MockSMF{}
	originalDispatcher := pfcp_dispatcher.Dispatcher
	pfcp_dispatcher.Dispatcher = pfcp_dispatcher.PfcpDispatcher{SMF: mockSMF}

	defer func() {
		pfcp_dispatcher.Dispatcher = originalDispatcher
	}()

	// Send multiple flow reports
	for i := range 5 {
		flow := ebpf.N3N6EntrypointFlow{
			Imsi:  1019756139935 + uint64(i),
			Saddr: makeIPUint32(192, 168, 1, 100),
			Daddr: makeIPUint32(8, 8, 8, 8),
			Sport: makePortUint16(uint16(10000 + i)),
			Dport: makePortUint16(53),
			Proto: 17,
		}

		stats := ebpf.N3N6EntrypointFlowStats{
			FirstTs: uint64(1000000000),
			LastTs:  uint64(1000300000),
			Packets: uint64(100 * (i + 1)),
			Bytes:   uint64(50000 * (i + 1)),
		}

		core.SendFlowReport(t.Context(), flow, stats)
	}

	if mockSMF.CallCount != 5 {
		t.Fatalf("Expected 5 calls to SendFlowReport, got %d", mockSMF.CallCount)
	}
}

func TestSendFlowReportDifferentProtocols(t *testing.T) {
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
			mockSMF := &MockSMF{}
			originalDispatcher := pfcp_dispatcher.Dispatcher
			pfcp_dispatcher.Dispatcher = pfcp_dispatcher.PfcpDispatcher{SMF: mockSMF}

			defer func() {
				pfcp_dispatcher.Dispatcher = originalDispatcher
			}()

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

			core.SendFlowReport(t.Context(), flow, stats)

			if mockSMF.LastFlowReport.Protocol != tc.protocol {
				t.Fatalf("Expected protocol %d, got %d", tc.protocol, mockSMF.LastFlowReport.Protocol)
			}

			if mockSMF.LastFlowReport.DestinationPort != tc.port {
				t.Fatalf("Expected port %d, got %d", tc.port, mockSMF.LastFlowReport.DestinationPort)
			}
		})
	}
}

func TestSendFlowReportTimestampFormatting(t *testing.T) {
	mockSMF := &MockSMF{}
	originalDispatcher := pfcp_dispatcher.Dispatcher
	pfcp_dispatcher.Dispatcher = pfcp_dispatcher.PfcpDispatcher{SMF: mockSMF}

	defer func() {
		pfcp_dispatcher.Dispatcher = originalDispatcher
	}()

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

	core.SendFlowReport(t.Context(), flow, stats)

	// Verify timestamps are in RFC3339 format
	report := mockSMF.LastFlowReport

	// Parse timestamps to verify they are valid RFC3339
	_, err := time.Parse(time.RFC3339, report.Timestamp)
	if err != nil {
		t.Fatalf("Invalid timestamp format: %s (error: %v)", report.Timestamp, err)
	}

	_, err = time.Parse(time.RFC3339, report.StartTime)
	if err != nil {
		t.Fatalf("Invalid start time format: %s (error: %v)", report.StartTime, err)
	}

	_, err = time.Parse(time.RFC3339, report.EndTime)
	if err != nil {
		t.Fatalf("Invalid end time format: %s (error: %v)", report.EndTime, err)
	}
}

func TestSendFlowReportIPAddressConversion(t *testing.T) {
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
			mockSMF := &MockSMF{}
			originalDispatcher := pfcp_dispatcher.Dispatcher
			pfcp_dispatcher.Dispatcher = pfcp_dispatcher.PfcpDispatcher{SMF: mockSMF}

			defer func() {
				pfcp_dispatcher.Dispatcher = originalDispatcher
			}()

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

			core.SendFlowReport(t.Context(), flow, stats)

			if mockSMF.LastFlowReport.SourceIP != tc.expectedSrc {
				t.Fatalf("Expected source IP %s, got %s", tc.expectedSrc, mockSMF.LastFlowReport.SourceIP)
			}

			if mockSMF.LastFlowReport.DestinationIP != tc.expectedDst {
				t.Fatalf("Expected destination IP %s, got %s", tc.expectedDst, mockSMF.LastFlowReport.DestinationIP)
			}
		})
	}
}

func TestSendFlowReportImsiFormatting(t *testing.T) {
	mockSMF := &MockSMF{}
	originalDispatcher := pfcp_dispatcher.Dispatcher
	pfcp_dispatcher.Dispatcher = pfcp_dispatcher.PfcpDispatcher{SMF: mockSMF}

	defer func() {
		pfcp_dispatcher.Dispatcher = originalDispatcher
	}()

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

	core.SendFlowReport(t.Context(), flow, stats)

	// Verify IMSI is formatted as 15-digit string
	if mockSMF.LastFlowReport.IMSI != "001019756139935" {
		t.Fatalf("Expected IMSI 001019756139935, got %s", mockSMF.LastFlowReport.IMSI)
	}
}
