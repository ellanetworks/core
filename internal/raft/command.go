// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// CommandType identifies a shared-DB write operation in the Raft log. Every
// member applies every committed entry, so adding or retiring a type needs a
// schema migration and db.RequireSchema(N) on the operation proposing it.
type CommandType uint16

const (
	CmdChangeset CommandType = 0

	// Retired ids are never reused: old logs and snapshots stay decodable.

	// Intent-based bulk deletes kept explicit for log-size control.
	CmdDeleteOldDailyUsage    CommandType = 12
	CmdDeleteAllDynamicLeases CommandType = 23
	CmdDeleteOldAuditLogs     CommandType = 31
	CmdDeleteExpiredSessions  CommandType = 72

	CmdMigrateShared CommandType = 220
)

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

// Command is the Raft log entry for shared-DB writes. Payloads are JSON:
// shared writes are low-volume config data, so being debuggable outweighs a
// protoc toolchain dependency.
type Command struct {
	Type    CommandType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func (c *Command) MarshalBinary() ([]byte, error) {
	var hdr [2]byte

	binary.BigEndian.PutUint16(hdr[:], uint16(c.Type))

	return append(hdr[:], c.Payload...), nil
}

func UnmarshalCommand(data []byte) (*Command, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("command too short: %d bytes", len(data))
	}

	return &Command{
		Type:    CommandType(binary.BigEndian.Uint16(data[:2])),
		Payload: json.RawMessage(data[2:]),
	}, nil
}

func NewCommand(cmdType CommandType, payload any) (*Command, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal %s payload: %w", cmdType, err)
	}

	return &Command{Type: cmdType, Payload: data}, nil
}

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
