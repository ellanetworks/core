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

type DBWriter interface {
	InsertRadioEvent(ctx context.Context, radioEvent *RadioEvent) error
	InsertAuditLog(ctx context.Context, auditLog *AuditLog) error
}
