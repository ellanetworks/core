// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package db_test

import (
	"context"
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

	// Create test flow report
	flowReport := &dbwriter.FlowReport{
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
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

	// Insert multiple flow reports
	now := time.Now().UTC()
	for i := range 5 {
		flowReport := &dbwriter.FlowReport{
			Timestamp:       now.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
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

	// Insert 15 flow reports
	now := time.Now().UTC()
	for i := range 15 {
		flowReport := &dbwriter.FlowReport{
			Timestamp:       now.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
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

	// Insert flow reports for different subscribers
	now := time.Now().UTC()
	subscribers := []string{"460123456789012", "460123456789013", "460123456789014"}

	for j, subscriber := range subscribers {
		for i := range 3 {
			flowReport := &dbwriter.FlowReport{
				Timestamp:       now.Add(time.Duration(j*3+i) * time.Minute).Format(time.RFC3339),
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

	// Insert flow reports with different protocols
	now := time.Now().UTC()
	protocols := []uint8{6, 17, 1} // TCP, UDP, ICMP

	for i, proto := range protocols {
		for j := range 2 {
			flowReport := &dbwriter.FlowReport{
				Timestamp:       now.Add(time.Duration(i*2+j) * time.Minute).Format(time.RFC3339),
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

	// Insert old flow report (10 days ago)
	oldTime := time.Now().UTC().Add(-10 * 24 * time.Hour)
	oldFlowReport := &dbwriter.FlowReport{
		Timestamp:       oldTime.Format(time.RFC3339),
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
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
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
