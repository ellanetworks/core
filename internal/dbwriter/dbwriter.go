package dbwriter

import "context"

type RadioEvent struct {
	ID            int    `db:"id"`
	Timestamp     string `db:"timestamp"` // store as RFC3339 string; parse in API layer if needed
	Protocol      string `db:"protocol"`
	MessageType   string `db:"message_type"`
	Direction     string `db:"direction"`
	LocalAddress  string `db:"local_address"`
	RemoteAddress string `db:"remote_address"`
	Raw           []byte `db:"raw"`
	Details       string `db:"details"` // JSON or plain text (we store a string)
}

type AuditLog struct {
	ID        int    `db:"id"`
	Timestamp string `db:"timestamp"` // store as RFC3339 string; parse in API layer if needed
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
	StartTime       string `db:"start_time"`       // RFC3339 first packet timestamp
	EndTime         string `db:"end_time"`         // RFC3339 last packet timestamp
	Direction       string `db:"direction"`        // "uplink" or "downlink"
}

type DBWriter interface {
	InsertRadioEvent(ctx context.Context, radioEvent *RadioEvent) error
	InsertFlowReport(ctx context.Context, flowReport *FlowReport) error
	InsertAuditLog(ctx context.Context, auditLog *AuditLog) error
}
