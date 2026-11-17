// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package dbwriter

import (
	"context"
	"time"
)

func EpochMillis(t time.Time) int64 {
	return t.UnixMilli()
}

func FromEpochMillis(ms int64) time.Time {
	return time.UnixMilli(ms).UTC()
}

func FormatEpochMillis(ms int64) string {
	return FromEpochMillis(ms).Format(time.RFC3339Nano)
}

type RadioEvent struct {
	ID            int
	Timestamp     int64 // epoch milliseconds (UTC)
	Protocol      string
	MessageType   string
	Direction     string
	LocalAddress  string
	RemoteAddress string
	RadioName     string
	RadioRef      string
	Raw           []byte
	Details       string // JSON or plain text (we store a string)
}

type AuditLog struct {
	ID        string // UUIDv7, generated at the request handler
	Timestamp int64  // epoch milliseconds (UTC)
	Level     string
	Actor     string
	Action    string
	IP        string
	Details   string // JSON or plain text (we store a string)
}

type FlowReport struct {
	ID              int
	SubscriberID    string // IMSI - looked up via PDR ID, not stored
	SourceIP        string // IPv4 or IPv6
	DestinationIP   string // IPv4 or IPv6
	SourcePort      uint16 // 0 if N/A (ICMP, etc.)
	DestinationPort uint16 // 0 if N/A
	Protocol        uint8  // IP protocol number (TCP=6, UDP=17, ICMP=1, etc.)
	Packets         uint64 // Total packets in flow
	Bytes           uint64 // Total bytes in flow
	StartTime       int64  // epoch milliseconds, first packet
	EndTime         int64  // epoch milliseconds, last packet
	Direction       string // "uplink" or "downlink"
	Action          int    // 0 = "allow", 1 = "deny"
}

type DBWriter interface {
	InsertRadioEvent(ctx context.Context, radioEvent *RadioEvent) error
	InsertFlowReports(ctx context.Context, flowReports []*FlowReport) error
	InsertAuditLog(ctx context.Context, auditLog *AuditLog) error
}
