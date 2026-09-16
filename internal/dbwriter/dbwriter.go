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
	ID            int    `db:"id"`
	Timestamp     int64  `db:"timestamp"` // epoch milliseconds (UTC)
	Protocol      string `db:"protocol"`
	MessageType   string `db:"message_type"`
	Direction     string `db:"direction"`
	LocalAddress  string `db:"local_address"`
	RemoteAddress string `db:"remote_address"`
	RadioName     string `db:"radio_name"`
	Raw           []byte `db:"raw"`
	Details       string `db:"details"` // JSON or plain text (we store a string)
}

type AuditLog struct {
	ID        string `db:"id"`        // UUIDv7, generated at the request handler
	Timestamp int64  `db:"timestamp"` // epoch milliseconds (UTC)
	Level     string `db:"level"`
	Actor     string `db:"actor"`
	Action    string `db:"action"`
	IP        string `db:"ip"`
	Details   string `db:"details"` // JSON or plain text (we store a string)
}

type FlowReport struct {
	ID              int    `db:"id"`
	SubscriberID    string `db:"subscriber_id"`    // IMSI - looked up via PDR ID, not stored
	SourceIP        string `db:"source_ip"`        // IPv4 or IPv6
	DestinationIP   string `db:"destination_ip"`   // IPv4 or IPv6
	SourcePort      uint16 `db:"source_port"`      // 0 if N/A (ICMP, etc.)
	DestinationPort uint16 `db:"destination_port"` // 0 if N/A
	Protocol        uint8  `db:"protocol"`         // IP protocol number (TCP=6, UDP=17, ICMP=1, etc.)
	Packets         uint64 `db:"packets"`          // Total packets in flow
	Bytes           uint64 `db:"bytes"`            // Total bytes in flow
	StartTime       int64  `db:"start_time"`       // epoch milliseconds, first packet
	EndTime         int64  `db:"end_time"`         // epoch milliseconds, last packet
	Direction       string `db:"direction"`        // "uplink" or "downlink"
	Action          int    `db:"action"`           // 0 = "allow", 1 = "deny"
}

type DBWriter interface {
	InsertRadioEvent(ctx context.Context, radioEvent *RadioEvent) error
	InsertFlowReports(ctx context.Context, flowReports []*FlowReport) error
	InsertAuditLog(ctx context.Context, auditLog *AuditLog) error
}
