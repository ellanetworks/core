// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package context

import "github.com/ellanetworks/core/internal/dbwriter"

// FlowReport represents flow statistics at the SMF layer before database persistence.
// This struct is independent of the dbwriter package and contains the complete flow data.
type FlowReport struct {
	SubscriberID    string // IMSI (populated after PDR lookup)
	Timestamp       string // RFC3339 when flow expired
	SourceIP        string // IPv4 or IPv6
	DestinationIP   string // IPv4 or IPv6
	SourcePort      uint16 // 0 if N/A (ICMP, etc.)
	DestinationPort uint16 // 0 if N/A
	Protocol        uint8  // IP protocol number (TCP=6, UDP=17, ICMP=1, etc.)
	Packets         uint64 // Total packets in flow
	Bytes           uint64 // Total bytes in flow
	StartTime       string // RFC3339 first packet timestamp
	EndTime         string // RFC3339 last packet timestamp
}

// ToDBWriter converts the SMF FlowReport to a database writer FlowReport for persistence.
func (fr *FlowReport) ToDBWriter() *dbwriter.FlowReport {
	return &dbwriter.FlowReport{
		Timestamp:       fr.Timestamp,
		SubscriberID:    fr.SubscriberID,
		SourceIP:        fr.SourceIP,
		DestinationIP:   fr.DestinationIP,
		SourcePort:      fr.SourcePort,
		DestinationPort: fr.DestinationPort,
		Protocol:        fr.Protocol,
		Packets:         fr.Packets,
		Bytes:           fr.Bytes,
		StartTime:       fr.StartTime,
		EndTime:         fr.EndTime,
	}
}
