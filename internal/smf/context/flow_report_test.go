// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package context_test

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/smf/context"
)

func TestFlowReportToDBWriter(t *testing.T) {
	now := time.Now().UTC()

	flowReport := &context.FlowReport{
		SubscriberID:    "460123456789012",
		Timestamp:       now.Format(time.RFC3339),
		SourceIP:        "192.168.1.100",
		DestinationIP:   "8.8.8.8",
		SourcePort:      12345,
		DestinationPort: 53,
		Protocol:        17, // UDP
		Packets:         1000,
		Bytes:           500000,
		StartTime:       now.Add(-5 * time.Minute).Format(time.RFC3339),
		EndTime:         now.Format(time.RFC3339),
	}

	// Convert to DBWriter format
	dbFlowReport := flowReport.ToDBWriter()

	// Verify all fields are preserved
	if dbFlowReport.SubscriberID != flowReport.SubscriberID {
		t.Fatalf("Expected subscriber ID %s, got %s", flowReport.SubscriberID, dbFlowReport.SubscriberID)
	}

	if dbFlowReport.SourceIP != flowReport.SourceIP {
		t.Fatalf("Expected source IP %s, got %s", flowReport.SourceIP, dbFlowReport.SourceIP)
	}

	if dbFlowReport.DestinationIP != flowReport.DestinationIP {
		t.Fatalf("Expected destination IP %s, got %s", flowReport.DestinationIP, dbFlowReport.DestinationIP)
	}

	if dbFlowReport.SourcePort != flowReport.SourcePort {
		t.Fatalf("Expected source port %d, got %d", flowReport.SourcePort, dbFlowReport.SourcePort)
	}

	if dbFlowReport.DestinationPort != flowReport.DestinationPort {
		t.Fatalf("Expected destination port %d, got %d", flowReport.DestinationPort, dbFlowReport.DestinationPort)
	}

	if dbFlowReport.Protocol != flowReport.Protocol {
		t.Fatalf("Expected protocol %d, got %d", flowReport.Protocol, dbFlowReport.Protocol)
	}

	if dbFlowReport.Packets != flowReport.Packets {
		t.Fatalf("Expected packets %d, got %d", flowReport.Packets, dbFlowReport.Packets)
	}

	if dbFlowReport.Bytes != flowReport.Bytes {
		t.Fatalf("Expected bytes %d, got %d", flowReport.Bytes, dbFlowReport.Bytes)
	}
}

func TestFlowReportToDBWriterPreservesTimestamps(t *testing.T) {
	startTime := "2026-02-23T10:00:00Z"
	endTime := "2026-02-23T10:05:00Z"
	timestamp := "2026-02-23T10:05:10Z"

	flowReport := &context.FlowReport{
		SubscriberID:    "460123456789012",
		Timestamp:       timestamp,
		SourceIP:        "192.168.1.100",
		DestinationIP:   "8.8.8.8",
		SourcePort:      12345,
		DestinationPort: 53,
		Protocol:        17,
		Packets:         100,
		Bytes:           50000,
		StartTime:       startTime,
		EndTime:         endTime,
	}

	dbFlowReport := flowReport.ToDBWriter()

	if dbFlowReport.Timestamp != timestamp {
		t.Fatalf("Expected timestamp %s, got %s", timestamp, dbFlowReport.Timestamp)
	}

	if dbFlowReport.StartTime != startTime {
		t.Fatalf("Expected start time %s, got %s", startTime, dbFlowReport.StartTime)
	}

	if dbFlowReport.EndTime != endTime {
		t.Fatalf("Expected end time %s, got %s", endTime, dbFlowReport.EndTime)
	}
}

func TestFlowReportToDBWriterDifferentProtocols(t *testing.T) {
	testCases := []struct {
		name     string
		protocol uint8
		portNum  uint16
	}{
		{"TCP", 6, 80},
		{"UDP", 17, 53},
		{"ICMP", 1, 0},
		{"GRE", 47, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flowReport := &context.FlowReport{
				SubscriberID:    "460123456789012",
				Timestamp:       time.Now().UTC().Format(time.RFC3339),
				SourceIP:        "192.168.1.100",
				DestinationIP:   "8.8.8.8",
				SourcePort:      12345,
				DestinationPort: tc.portNum,
				Protocol:        tc.protocol,
				Packets:         100,
				Bytes:           50000,
				StartTime:       time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339),
				EndTime:         time.Now().UTC().Format(time.RFC3339),
			}

			dbFlowReport := flowReport.ToDBWriter()

			if dbFlowReport.Protocol != tc.protocol {
				t.Fatalf("Expected protocol %d, got %d", tc.protocol, dbFlowReport.Protocol)
			}

			if dbFlowReport.DestinationPort != tc.portNum {
				t.Fatalf("Expected port %d, got %d", tc.portNum, dbFlowReport.DestinationPort)
			}
		})
	}
}
