// Copyright 2026 Ella Networks

package raft

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// CommandType identifies a shared-DB write operation in the Raft log.
// Each type maps to exactly one applyX function in the FSM.
type CommandType uint16

const (
	// Changeset replication (opaque sqlite3session bytes)
	CmdChangeset CommandType = 0

	// Gaps are intentional: retired command ids are never reused so any
	// historical logs/snapshots remain decodable.

	// Intent-based bulk deletes kept explicit for log-size control.
	CmdDeleteOldDailyUsage    CommandType = 12
	CmdDeleteAllDynamicLeases CommandType = 23
	CmdDeleteOldAuditLogs     CommandType = 31
	CmdDeleteExpiredSessions  CommandType = 72

	// Migrations — proposed by the leader to advance the shared schema
	CmdMigrateShared CommandType = 220
)

// commandNames provides human-readable names for logging and debugging.
var commandNames = map[CommandType]string{
	CmdChangeset:              "Changeset",
	CmdDeleteOldDailyUsage:    "DeleteOldDailyUsage",
	CmdDeleteAllDynamicLeases: "DeleteAllDynamicLeases",
	CmdDeleteOldAuditLogs:     "DeleteOldAuditLogs",
	CmdDeleteExpiredSessions:  "DeleteExpiredSessions",
	CmdMigrateShared:          "MigrateShared",
}

func (c CommandType) String() string {
	if name, ok := commandNames[c]; ok {
		return name
	}

	return fmt.Sprintf("CommandType(%d)", c)
}

// Command is the Raft log entry for shared-DB writes.
//
// Wire format:
//
//	[0:2]  CommandType (uint16, big-endian)
//	[2:]   JSON-encoded payload
//
// JSON is used for payloads because shared writes are low-volume (tens/sec)
// and payloads are small configuration data. This avoids a protoc toolchain
// dependency while remaining self-describing and debuggable.
type Command struct {
	Type    CommandType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// MarshalBinary encodes the command into the wire format.
func (c *Command) MarshalBinary() ([]byte, error) {
	var hdr [2]byte

	binary.BigEndian.PutUint16(hdr[:], uint16(c.Type))

	return append(hdr[:], c.Payload...), nil
}

// UnmarshalCommand decodes a command from the wire format.
func UnmarshalCommand(data []byte) (*Command, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("command too short: %d bytes", len(data))
	}

	return &Command{
		Type:    CommandType(binary.BigEndian.Uint16(data[:2])),
		Payload: json.RawMessage(data[2:]),
	}, nil
}

// NewCommand creates a command with the given type and JSON-serialized payload.
func NewCommand(cmdType CommandType, payload any) (*Command, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal %s payload: %w", cmdType, err)
	}

	return &Command{Type: cmdType, Payload: data}, nil
}

// Label returns a human-readable label for the command including the
// operation name embedded in changeset payloads (e.g. "Changeset(UpsertClusterMember)").
func (c *Command) Label() string {
	name := c.Type.String()
	if c.Type != CmdChangeset || len(c.Payload) == 0 {
		return name
	}

	var meta struct {
		Operation string `json:"operation"`
	}

	if err := json.Unmarshal(c.Payload, &meta); err == nil && meta.Operation != "" {
		return name + "(" + meta.Operation + ")"
	}

	return name
}
