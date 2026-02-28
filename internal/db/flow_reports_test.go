// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package db_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
)

func TestFlowReportsInsertAndRetrieve(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	ctx := context.Background()

	_, err = createDataNetworkPolicyAndSubscriber(database, "460123456789012")
	if err != nil {
		t.Fatalf("couldn't create prerequisite subscriber: %s", err)
	}

	// Create test flow report
	flowReport := &dbwriter.FlowReport{
		SubscriberID:    "460123456789012",
		SourceIP:        "192.168.1.100",
		DestinationIP:   "8.8.8.8",
		SourcePort:      12345,
		DestinationPort: 53,
		Protocol:        17, // UDP
		Packets:         100,
		Bytes:           50000,
		StartTime:       time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339),
		EndTime:         time.Now().UTC().Format(time.RFC3339),
		Direction:       "uplink",
	}

	// Insert flow report
	err = database.InsertFlowReport(ctx, flowReport)
	if err != nil {
		t.Fatalf("couldn't insert flow report: %s", err)
	}

	// List flow reports
	reports, total, err := database.ListFlowReports(ctx, 1, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list flow reports: %s", err)
	}

	if total != 1 {
		t.Fatalf("Expected total count to be 1, but got %d", total)
	}

	if len(reports) != 1 {
		t.Fatalf("Expected 1 flow report, but found %d", len(reports))
	}

	// Verify data
	if reports[0].SubscriberID != flowReport.SubscriberID {
		t.Fatalf("Expected subscriber ID %s, but got %s", flowReport.SubscriberID, reports[0].SubscriberID)
	}

	if reports[0].SourceIP != flowReport.SourceIP {
		t.Fatalf("Expected source IP %s, but got %s", flowReport.SourceIP, reports[0].SourceIP)
	}

	if reports[0].Protocol != flowReport.Protocol {
		t.Fatalf("Expected protocol %d, but got %d", flowReport.Protocol, reports[0].Protocol)
	}

	if reports[0].Packets != flowReport.Packets {
		t.Fatalf("Expected packets %d, but got %d", flowReport.Packets, reports[0].Packets)
	}

	if reports[0].Bytes != flowReport.Bytes {
		t.Fatalf("Expected bytes %d, but got %d", flowReport.Bytes, reports[0].Bytes)
	}

	if reports[0].Direction != flowReport.Direction {
		t.Fatalf("Expected direction %s, but got %s", flowReport.Direction, reports[0].Direction)
	}
}

func TestFlowReportsMultipleInsert(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	ctx := context.Background()

	_, err = createDataNetworkPolicyAndSubscriber(database, "460123456789012")
	if err != nil {
		t.Fatalf("couldn't create prerequisite subscriber: %s", err)
	}

	// Insert multiple flow reports
	now := time.Now().UTC()
	for i := range 5 {
		flowReport := &dbwriter.FlowReport{
			SubscriberID:    "460123456789012",
			SourceIP:        "192.168.1.100",
			DestinationIP:   "8.8.8.8",
			SourcePort:      uint16(12345 + i),
			DestinationPort: 53,
			Protocol:        17,
			Packets:         uint64(100 * (i + 1)),
			Bytes:           uint64(50000 * (i + 1)),
			StartTime:       now.Add(time.Duration(i)*time.Minute - 5*time.Minute).Format(time.RFC3339),
			EndTime:         now.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
		}

		err := database.InsertFlowReport(ctx, flowReport)
		if err != nil {
			t.Fatalf("couldn't insert flow report %d: %s", i, err)
		}
	}

	// List flow reports
	reports, total, err := database.ListFlowReports(ctx, 1, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list flow reports: %s", err)
	}

	if total != 5 {
		t.Fatalf("Expected total count to be 5, but got %d", total)
	}

	if len(reports) != 5 {
		t.Fatalf("Expected 5 flow reports, but found %d", len(reports))
	}
}

func TestFlowReportsPagination(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	ctx := context.Background()

	_, err = createDataNetworkPolicyAndSubscriber(database, "460123456789012")
	if err != nil {
		t.Fatalf("couldn't create prerequisite subscriber: %s", err)
	}

	// Insert 15 flow reports
	now := time.Now().UTC()
	for i := range 15 {
		flowReport := &dbwriter.FlowReport{
			SubscriberID:    "460123456789012",
			SourceIP:        "192.168.1.100",
			DestinationIP:   "8.8.8.8",
			SourcePort:      uint16(12345 + i),
			DestinationPort: 53,
			Protocol:        17,
			Packets:         100,
			Bytes:           50000,
			StartTime:       now.Add(time.Duration(i)*time.Minute - 5*time.Minute).Format(time.RFC3339),
			EndTime:         now.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
		}

		err := database.InsertFlowReport(ctx, flowReport)
		if err != nil {
			t.Fatalf("couldn't insert flow report: %s", err)
		}
	}

	// Test first page
	reports, total, err := database.ListFlowReports(ctx, 1, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list flow reports page 1: %s", err)
	}

	if total != 15 {
		t.Fatalf("Expected total 15, got %d", total)
	}

	if len(reports) != 10 {
		t.Fatalf("Expected 10 reports on page 1, got %d", len(reports))
	}

	// Test second page
	reports, total, err = database.ListFlowReports(ctx, 2, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list flow reports page 2: %s", err)
	}

	if total != 15 {
		t.Fatalf("Expected total 15, got %d", total)
	}

	if len(reports) != 5 {
		t.Fatalf("Expected 5 reports on page 2, got %d", len(reports))
	}
}

func TestFlowReportsFilterBySubscriber(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	ctx := context.Background()

	// Create subscribers
	policyID, err := createDataNetworkPolicyAndSubscriber(database, "460123456789012")
	if err != nil {
		t.Fatalf("couldn't create prerequisite subscriber: %s", err)
	}

	for _, imsi := range []string{"460123456789013", "460123456789014"} {
		subscriber := &db.Subscriber{
			Imsi:           imsi,
			SequenceNumber: "000000000022",
			PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
			Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
			PolicyID:       policyID,
		}

		err = database.CreateSubscriber(ctx, subscriber)
		if err != nil {
			t.Fatalf("couldn't create subscriber %s: %s", imsi, err)
		}
	}

	// Insert flow reports for different subscribers
	now := time.Now().UTC()
	subscribers := []string{"460123456789012", "460123456789013", "460123456789014"}

	for j, subscriber := range subscribers {
		for i := range 3 {
			flowReport := &dbwriter.FlowReport{
				SubscriberID:    subscriber,
				SourceIP:        "192.168.1.100",
				DestinationIP:   "8.8.8.8",
				SourcePort:      12345,
				DestinationPort: 53,
				Protocol:        17,
				Packets:         100,
				Bytes:           50000,
				StartTime:       now.Add(time.Duration(j*3+i)*time.Minute - 5*time.Minute).Format(time.RFC3339),
				EndTime:         now.Add(time.Duration(j*3+i) * time.Minute).Format(time.RFC3339),
			}

			err := database.InsertFlowReport(ctx, flowReport)
			if err != nil {
				t.Fatalf("couldn't insert flow report: %s", err)
			}
		}
	}

	// Filter by subscriber
	subscriber := "460123456789012"
	filter := &db.FlowReportFilters{
		SubscriberID: &subscriber,
	}

	reports, total, err := database.ListFlowReports(ctx, 1, 10, filter)
	if err != nil {
		t.Fatalf("couldn't list filtered flow reports: %s", err)
	}

	if total != 3 {
		t.Fatalf("Expected total 3 for subscriber, got %d", total)
	}

	if len(reports) != 3 {
		t.Fatalf("Expected 3 reports for subscriber, got %d", len(reports))
	}

	for _, report := range reports {
		if report.SubscriberID != "460123456789012" {
			t.Fatalf("Expected subscriber 460123456789012, got %s", report.SubscriberID)
		}
	}
}

func TestFlowReportsFilterByProtocol(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	ctx := context.Background()

	_, err = createDataNetworkPolicyAndSubscriber(database, "460123456789012")
	if err != nil {
		t.Fatalf("couldn't create prerequisite subscriber: %s", err)
	}

	// Insert flow reports with different protocols
	now := time.Now().UTC()
	protocols := []uint8{6, 17, 1} // TCP, UDP, ICMP

	for i, proto := range protocols {
		for j := range 2 {
			flowReport := &dbwriter.FlowReport{
				SubscriberID:    "460123456789012",
				SourceIP:        "192.168.1.100",
				DestinationIP:   "8.8.8.8",
				SourcePort:      12345,
				DestinationPort: 53,
				Protocol:        proto,
				Packets:         100,
				Bytes:           50000,
				StartTime:       now.Add(time.Duration(i*2+j)*time.Minute - 5*time.Minute).Format(time.RFC3339),
				EndTime:         now.Add(time.Duration(i*2+j) * time.Minute).Format(time.RFC3339),
			}

			err := database.InsertFlowReport(ctx, flowReport)
			if err != nil {
				t.Fatalf("couldn't insert flow report: %s", err)
			}
		}
	}

	// Filter by protocol (UDP = 17)
	proto := uint8(17)
	filter := &db.FlowReportFilters{
		Protocol: &proto,
	}

	reports, total, err := database.ListFlowReports(ctx, 1, 10, filter)
	if err != nil {
		t.Fatalf("couldn't list filtered flow reports: %s", err)
	}

	if total != 2 {
		t.Fatalf("Expected total 2 for UDP protocol, got %d", total)
	}

	if len(reports) != 2 {
		t.Fatalf("Expected 2 reports for UDP protocol, got %d", len(reports))
	}

	for _, report := range reports {
		if report.Protocol != 17 {
			t.Fatalf("Expected protocol 17, got %d", report.Protocol)
		}
	}
}

func TestGetFlowReportStats_Empty(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	ctx := context.Background()

	protocols, topDstUplink, err := database.GetFlowReportStats(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error from GetFlowReportStats: %s", err)
	}

	if len(protocols) != 0 {
		t.Fatalf("expected 0 protocol counts, got %d", len(protocols))
	}

	if len(topDstUplink) != 0 {
		t.Fatalf("expected 0 top destinations uplink, got %d", len(topDstUplink))
	}
}

func TestGetFlowReportStats_ProtocolCounts(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	ctx := context.Background()

	_, err = createDataNetworkPolicyAndSubscriber(database, "460123456789012")
	if err != nil {
		t.Fatalf("couldn't create prerequisite subscriber: %s", err)
	}

	// Insert 3 TCP, 2 UDP, 1 ICMP
	now := time.Now().UTC()
	protoCounts := []struct {
		proto uint8
		count int
	}{
		{6, 3},  // TCP
		{17, 2}, // UDP
		{1, 1},  // ICMP
	}

	idx := 0

	for _, pc := range protoCounts {
		for range pc.count {
			fr := &dbwriter.FlowReport{
				SubscriberID:    "460123456789012",
				SourceIP:        "10.0.0.1",
				DestinationIP:   "8.8.8.8",
				SourcePort:      uint16(10000 + idx),
				DestinationPort: 80,
				Protocol:        pc.proto,
				Packets:         10,
				Bytes:           1000,
				StartTime:       now.Add(time.Duration(idx)*time.Minute - 5*time.Minute).Format(time.RFC3339),
				EndTime:         now.Add(time.Duration(idx) * time.Minute).Format(time.RFC3339),
			}
			if err := database.InsertFlowReport(ctx, fr); err != nil {
				t.Fatalf("couldn't insert flow report: %s", err)
			}

			idx++
		}
	}

	protocols, _, err := database.GetFlowReportStats(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error from GetFlowReportStats: %s", err)
	}

	if len(protocols) != 3 {
		t.Fatalf("expected 3 protocol entries, got %d", len(protocols))
	}

	// Results are ordered by count DESC: TCP(3), UDP(2), ICMP(1)
	if protocols[0].Protocol != 6 {
		t.Fatalf("expected first protocol to be TCP (6), got %d", protocols[0].Protocol)
	}

	if protocols[0].Count != 3 {
		t.Fatalf("expected TCP count to be 3, got %d", protocols[0].Count)
	}

	if protocols[1].Protocol != 17 {
		t.Fatalf("expected second protocol to be UDP (17), got %d", protocols[1].Protocol)
	}

	if protocols[1].Count != 2 {
		t.Fatalf("expected UDP count to be 2, got %d", protocols[1].Count)
	}

	if protocols[2].Protocol != 1 {
		t.Fatalf("expected third protocol to be ICMP (1), got %d", protocols[2].Protocol)
	}

	if protocols[2].Count != 1 {
		t.Fatalf("expected ICMP count to be 1, got %d", protocols[2].Count)
	}
}

func TestGetFlowReportStats_WithSubscriberFilter(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	ctx := context.Background()

	policyID, err := createDataNetworkPolicyAndSubscriber(database, "460123456789012")
	if err != nil {
		t.Fatalf("couldn't create prerequisite subscriber: %s", err)
	}

	sub2 := &db.Subscriber{
		Imsi:           "460123456789013",
		SequenceNumber: "000000000022",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		PolicyID:       policyID,
	}
	if err := database.CreateSubscriber(ctx, sub2); err != nil {
		t.Fatalf("couldn't create subscriber: %s", err)
	}

	now := time.Now().UTC()

	// Insert 3 TCP flows for subscriber 1
	for i := range 3 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "460123456789012",
			SourceIP:        "10.0.0.1",
			DestinationIP:   "1.1.1.1",
			SourcePort:      uint16(10000 + i),
			DestinationPort: 443,
			Protocol:        6,
			Packets:         10,
			Bytes:           1000,
			StartTime:       now.Add(time.Duration(i)*time.Minute - 5*time.Minute).Format(time.RFC3339),
			EndTime:         now.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
		}
		if err := database.InsertFlowReport(ctx, fr); err != nil {
			t.Fatalf("couldn't insert flow report: %s", err)
		}
	}

	// Insert 2 UDP flows for subscriber 2
	for i := range 2 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "460123456789013",
			SourceIP:        "10.0.0.2",
			DestinationIP:   "2.2.2.2",
			SourcePort:      uint16(20000 + i),
			DestinationPort: 53,
			Protocol:        17,
			Packets:         5,
			Bytes:           500,
			StartTime:       now.Add(time.Duration(3+i)*time.Minute - 5*time.Minute).Format(time.RFC3339),
			EndTime:         now.Add(time.Duration(3+i) * time.Minute).Format(time.RFC3339),
		}
		if err := database.InsertFlowReport(ctx, fr); err != nil {
			t.Fatalf("couldn't insert flow report: %s", err)
		}
	}

	sub1 := "460123456789012"
	filter := &db.FlowReportFilters{SubscriberID: &sub1}

	protocols, _, err := database.GetFlowReportStats(ctx, filter)
	if err != nil {
		t.Fatalf("unexpected error from GetFlowReportStats: %s", err)
	}

	// Only subscriber 1's TCP flows should be counted
	if len(protocols) != 1 {
		t.Fatalf("expected 1 protocol entry, got %d", len(protocols))
	}

	if protocols[0].Protocol != 6 {
		t.Fatalf("expected protocol TCP (6), got %d", protocols[0].Protocol)
	}

	if protocols[0].Count != 3 {
		t.Fatalf("expected protocol count 3, got %d", protocols[0].Count)
	}
}

func TestGetFlowReportStats_WithProtocolFilter(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	ctx := context.Background()

	_, err = createDataNetworkPolicyAndSubscriber(database, "460123456789012")
	if err != nil {
		t.Fatalf("couldn't create prerequisite subscriber: %s", err)
	}

	now := time.Now().UTC()

	// Insert 4 TCP flows with distinct source IPs
	for i := range 4 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "460123456789012",
			SourceIP:        fmt.Sprintf("192.168.1.%d", i+1),
			DestinationIP:   "8.8.8.8",
			SourcePort:      uint16(10000 + i),
			DestinationPort: 443,
			Protocol:        6,
			Packets:         10,
			Bytes:           1000,
			StartTime:       now.Add(time.Duration(i)*time.Minute - 5*time.Minute).Format(time.RFC3339),
			EndTime:         now.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
		}
		if err := database.InsertFlowReport(ctx, fr); err != nil {
			t.Fatalf("couldn't insert TCP flow report: %s", err)
		}
	}

	// Insert 2 UDP flows
	for i := range 2 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "460123456789012",
			SourceIP:        fmt.Sprintf("10.0.0.%d", i+1),
			DestinationIP:   "1.1.1.1",
			SourcePort:      uint16(20000 + i),
			DestinationPort: 53,
			Protocol:        17,
			Packets:         5,
			Bytes:           500,
			StartTime:       now.Add(time.Duration(4+i)*time.Minute - 5*time.Minute).Format(time.RFC3339),
			EndTime:         now.Add(time.Duration(4+i) * time.Minute).Format(time.RFC3339),
		}
		if err := database.InsertFlowReport(ctx, fr); err != nil {
			t.Fatalf("couldn't insert UDP flow report: %s", err)
		}
	}

	proto := uint8(6) // TCP
	filter := &db.FlowReportFilters{Protocol: &proto}

	protocols, _, err := database.GetFlowReportStats(ctx, filter)
	if err != nil {
		t.Fatalf("unexpected error from GetFlowReportStats: %s", err)
	}

	// Only TCP flows should appear
	if len(protocols) != 1 {
		t.Fatalf("expected 1 protocol entry, got %d", len(protocols))
	}

	if protocols[0].Protocol != 6 {
		t.Fatalf("expected protocol TCP (6), got %d", protocols[0].Protocol)
	}

	if protocols[0].Count != 4 {
		t.Fatalf("expected TCP count 4, got %d", protocols[0].Count)
	}
}

func TestGetFlowReportStats_WithDateFilter(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	ctx := context.Background()

	_, err = createDataNetworkPolicyAndSubscriber(database, "460123456789012")
	if err != nil {
		t.Fatalf("couldn't create prerequisite subscriber: %s", err)
	}

	now := time.Now().UTC()

	// Insert 3 recent flows (within last hour)
	for i := range 3 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "460123456789012",
			SourceIP:        "10.0.0.1",
			DestinationIP:   "8.8.8.8",
			SourcePort:      uint16(10000 + i),
			DestinationPort: 443,
			Protocol:        6,
			Packets:         10,
			Bytes:           1000,
			StartTime:       now.Add(-10 * time.Minute).Format(time.RFC3339),
			EndTime:         now.Add(-time.Duration(i) * time.Minute).Format(time.RFC3339),
		}
		if err := database.InsertFlowReport(ctx, fr); err != nil {
			t.Fatalf("couldn't insert recent flow report: %s", err)
		}
	}

	// Insert 2 old flows (2 days ago)
	oldTime := now.AddDate(0, 0, -2)
	for i := range 2 {
		fr := &dbwriter.FlowReport{
			SubscriberID:    "460123456789012",
			SourceIP:        "172.16.0.1",
			DestinationIP:   "1.1.1.1",
			SourcePort:      uint16(20000 + i),
			DestinationPort: 53,
			Protocol:        17,
			Packets:         5,
			Bytes:           500,
			StartTime:       oldTime.Add(-5 * time.Minute).Format(time.RFC3339),
			EndTime:         oldTime.Format(time.RFC3339),
		}
		if err := database.InsertFlowReport(ctx, fr); err != nil {
			t.Fatalf("couldn't insert old flow report: %s", err)
		}
	}

	// Filter to only yesterday onwards (excludes the 2-day-old flows)
	from := now.AddDate(0, 0, -1).Format(time.RFC3339)
	to := now.AddDate(0, 0, 1).Format(time.RFC3339)
	filter := &db.FlowReportFilters{
		EndTimeFrom: &from,
		EndTimeTo:   &to,
	}

	protocols, _, err := database.GetFlowReportStats(ctx, filter)
	if err != nil {
		t.Fatalf("unexpected error from GetFlowReportStats: %s", err)
	}

	// Only the 3 recent TCP flows should be counted
	if len(protocols) != 1 {
		t.Fatalf("expected 1 protocol entry, got %d", len(protocols))
	}

	if protocols[0].Protocol != 6 {
		t.Fatalf("expected protocol TCP (6), got %d", protocols[0].Protocol)
	}

	if protocols[0].Count != 3 {
		t.Fatalf("expected count 3, got %d", protocols[0].Count)
	}
}

func TestFlowReportsRetention(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	ctx := context.Background()

	_, err = createDataNetworkPolicyAndSubscriber(database, "460123456789012")
	if err != nil {
		t.Fatalf("couldn't create prerequisite subscriber: %s", err)
	}

	// Insert old flow report (10 days ago)
	oldTime := time.Now().UTC().Add(-10 * 24 * time.Hour)
	oldFlowReport := &dbwriter.FlowReport{
		SubscriberID:    "460123456789012",
		SourceIP:        "192.168.1.100",
		DestinationIP:   "8.8.8.8",
		SourcePort:      12345,
		DestinationPort: 53,
		Protocol:        17,
		Packets:         100,
		Bytes:           50000,
		StartTime:       oldTime.Add(-5 * time.Minute).Format(time.RFC3339),
		EndTime:         oldTime.Format(time.RFC3339),
	}

	err = database.InsertFlowReport(ctx, oldFlowReport)
	if err != nil {
		t.Fatalf("couldn't insert old flow report: %s", err)
	}

	// Insert recent flow report
	recentFlowReport := &dbwriter.FlowReport{
		SubscriberID:    "460123456789012",
		SourceIP:        "192.168.1.100",
		DestinationIP:   "8.8.8.8",
		SourcePort:      12345,
		DestinationPort: 53,
		Protocol:        17,
		Packets:         100,
		Bytes:           50000,
		StartTime:       time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339),
		EndTime:         time.Now().UTC().Format(time.RFC3339),
	}

	err = database.InsertFlowReport(ctx, recentFlowReport)
	if err != nil {
		t.Fatalf("couldn't insert recent flow report: %s", err)
	}

	// Verify both exist
	reports, total, err := database.ListFlowReports(ctx, 1, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list flow reports: %s", err)
	}

	if total != 2 {
		t.Fatalf("Expected 2 flow reports before retention, got %d", total)
	}

	if len(reports) != 2 {
		t.Fatalf("Expected 2 flow reports before retention, got %d", len(reports))
	}

	// Run retention (7 day retention)
	err = database.DeleteOldFlowReports(ctx, 7)
	if err != nil {
		t.Fatalf("couldn't run retention: %s", err)
	}

	// Verify only recent report remains
	reports, total, err = database.ListFlowReports(ctx, 1, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list flow reports after retention: %s", err)
	}

	if total != 1 {
		t.Fatalf("Expected 1 flow report after retention, got %d", total)
	}

	if reports[0].SubscriberID != recentFlowReport.SubscriberID {
		t.Fatalf("Wrong report retained")
	}
}
