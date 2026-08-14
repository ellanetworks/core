// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// CommandType identifies a shared-DB write operation in the Raft log.
//
// A new type ships with a schema migration, and the operation proposing
// it declares db.RequireSchema(N) for that migration's version. A
// migration applies only once every member reports a binary supporting
// it, and the join handshake holds that floor, making the applied schema
// a durable cluster-wide statement of which commands every member can
// apply.
type CommandType uint16

const (
	CmdChangeset CommandType = 0

	// Retired ids are never reused, keeping old logs and snapshots
	// decodable. Decoding is not applying, so retiring an id is gated
	// like adding one.

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

// Command is the Raft log entry for shared-DB writes. Payloads are JSON
// because shared writes are low-volume configuration data, where being
// self-describing and debuggable outweighs a protoc toolchain dependency.
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

// Label renders a command as e.g. "Changeset(UpsertClusterMember)".
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
