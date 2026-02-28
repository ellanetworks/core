// SPDX-FileCopyrightText: 2025-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package pfcp_dispatcher

import (
	"context"

	"github.com/wmnsk/go-pfcp/message"
)

// FlowReportRequest is sent by UPF to SMF with flow statistics (bypasses PFCP wire format)
// The SMF will convert this to a database representation and persist it.
type FlowReportRequest struct {
	IMSI            string // Subscriber IMSI (required, used to identify subscriber)
	SourceIP        string // IPv4 or IPv6
	DestinationIP   string // IPv4 or IPv6
	SourcePort      uint16 // 0 if N/A (ICMP, etc.)
	DestinationPort uint16 // 0 if N/A
	Protocol        uint8  // IP protocol number (TCP=6, UDP=17, ICMP=1, etc.)
	Packets         uint64 // Total packets in flow
	Bytes           uint64 // Total bytes in flow
	StartTime       string // RFC3339 first packet timestamp
	EndTime         string // RFC3339 last packet timestamp
	Direction       string // "uplink" or "downlink"
}

type UPF interface {
	HandlePfcpSessionEstablishmentRequest(context.Context, *message.SessionEstablishmentRequest) (*message.SessionEstablishmentResponse, error)
	HandlePfcpSessionDeletionRequest(context.Context, *message.SessionDeletionRequest) (*message.SessionDeletionResponse, error)
	HandlePfcpSessionModificationRequest(context.Context, *message.SessionModificationRequest) (*message.SessionModificationResponse, error)
}

type SMF interface {
	HandlePfcpSessionReportRequest(context.Context, *message.SessionReportRequest) (*message.SessionReportResponse, error)
	SendFlowReport(context.Context, *FlowReportRequest) error
}

type PfcpDispatcher struct {
	SMF SMF
	UPF UPF
}

var Dispatcher PfcpDispatcher

func NewPfcpDispatcher(smf SMF, upf UPF) PfcpDispatcher {
	return PfcpDispatcher{SMF: smf, UPF: upf}
}
