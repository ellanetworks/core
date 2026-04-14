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

	// Subscribers
	CmdCreateSubscriber        CommandType = 1
	CmdUpdateSubscriberProfile CommandType = 2
	CmdEditSubscriberSeqNum    CommandType = 3
	CmdDeleteSubscriber        CommandType = 4

	// Daily Usage
	CmdIncrementDailyUsage CommandType = 10
	CmdClearDailyUsage     CommandType = 11
	CmdDeleteOldDailyUsage CommandType = 12

	// IP Leases
	CmdCreateLease               CommandType = 20
	CmdUpdateLeaseSession        CommandType = 21
	CmdDeleteDynamicLease        CommandType = 22
	CmdDeleteAllDynamicLeases    CommandType = 23
	CmdDeleteDynamicLeasesByNode CommandType = 24
	CmdUpdateLeaseNode           CommandType = 25

	// Audit Logs
	CmdInsertAuditLog     CommandType = 30
	CmdDeleteOldAuditLogs CommandType = 31

	// Users
	CmdCreateUser         CommandType = 40
	CmdUpdateUser         CommandType = 41
	CmdUpdateUserPassword CommandType = 42
	CmdDeleteUser         CommandType = 43

	// Profiles
	CmdCreateProfile CommandType = 50
	CmdUpdateProfile CommandType = 51
	CmdDeleteProfile CommandType = 52

	// API Tokens
	CmdCreateAPIToken CommandType = 60
	CmdDeleteAPIToken CommandType = 61

	// Sessions
	CmdCreateSession            CommandType = 70
	CmdDeleteSessionByTokenHash CommandType = 71
	CmdDeleteExpiredSessions    CommandType = 72
	CmdDeleteOldestSessions     CommandType = 73
	CmdDeleteAllSessionsForUser CommandType = 74
	CmdDeleteAllSessions        CommandType = 75

	// Network Slices
	CmdCreateNetworkSlice CommandType = 80
	CmdUpdateNetworkSlice CommandType = 81
	CmdDeleteNetworkSlice CommandType = 82

	// Data Networks
	CmdCreateDataNetwork CommandType = 90
	CmdUpdateDataNetwork CommandType = 91
	CmdDeleteDataNetwork CommandType = 92

	// Policies
	CmdCreatePolicy CommandType = 100
	CmdUpdatePolicy CommandType = 101
	CmdDeletePolicy CommandType = 102

	// Network Rules
	CmdCreateNetworkRule          CommandType = 110
	CmdUpdateNetworkRule          CommandType = 111
	CmdDeleteNetworkRule          CommandType = 112
	CmdDeleteNetworkRulesByPolicy CommandType = 113

	// Home Network Keys
	CmdCreateHomeNetworkKey CommandType = 120
	CmdDeleteHomeNetworkKey CommandType = 121

	// BGP Peers
	CmdCreateBGPPeer CommandType = 130
	CmdUpdateBGPPeer CommandType = 131
	CmdDeleteBGPPeer CommandType = 132

	// BGP Settings
	CmdUpdateBGPSettings CommandType = 140

	// BGP Import Prefixes
	CmdSetImportPrefixesForPeer CommandType = 150

	// Settings (singletons)
	CmdUpdateNATSettings            CommandType = 160
	CmdUpdateN3Settings             CommandType = 161
	CmdUpdateFlowAccountingSettings CommandType = 162
	CmdSetRetentionPolicy           CommandType = 163

	// Operator
	CmdInitializeOperator               CommandType = 170
	CmdUpdateOperatorTracking           CommandType = 171
	CmdUpdateOperatorID                 CommandType = 172
	CmdUpdateOperatorCode               CommandType = 173
	CmdUpdateOperatorSecurityAlgorithms CommandType = 174
	CmdUpdateOperatorSPN                CommandType = 175
	CmdUpdateOperatorAMFIdentity        CommandType = 176
	CmdUpdateOperatorClusterID          CommandType = 177

	// JWT Secret
	CmdSetJWTSecret CommandType = 180

	// Routes
	CmdCreateRoute CommandType = 190
	CmdDeleteRoute CommandType = 191

	// Cluster Members
	CmdUpsertClusterMember CommandType = 200
	CmdDeleteClusterMember CommandType = 201

	// Migrations — proposed by the leader to advance the shared schema
	CmdMigrateShared CommandType = 220

	// Restore — special: replaces the entire shared.db
	CmdRestore CommandType = 255
)

// commandNames provides human-readable names for logging and debugging.
var commandNames = map[CommandType]string{
	CmdChangeset:                        "Changeset",
	CmdCreateSubscriber:                 "CreateSubscriber",
	CmdUpdateSubscriberProfile:          "UpdateSubscriberProfile",
	CmdEditSubscriberSeqNum:             "EditSubscriberSeqNum",
	CmdDeleteSubscriber:                 "DeleteSubscriber",
	CmdIncrementDailyUsage:              "IncrementDailyUsage",
	CmdClearDailyUsage:                  "ClearDailyUsage",
	CmdDeleteOldDailyUsage:              "DeleteOldDailyUsage",
	CmdCreateLease:                      "CreateLease",
	CmdUpdateLeaseSession:               "UpdateLeaseSession",
	CmdDeleteDynamicLease:               "DeleteDynamicLease",
	CmdDeleteAllDynamicLeases:           "DeleteAllDynamicLeases",
	CmdDeleteDynamicLeasesByNode:        "DeleteDynamicLeasesByNode",
	CmdUpdateLeaseNode:                  "UpdateLeaseNode",
	CmdInsertAuditLog:                   "InsertAuditLog",
	CmdDeleteOldAuditLogs:               "DeleteOldAuditLogs",
	CmdCreateUser:                       "CreateUser",
	CmdUpdateUser:                       "UpdateUser",
	CmdUpdateUserPassword:               "UpdateUserPassword",
	CmdDeleteUser:                       "DeleteUser",
	CmdCreateProfile:                    "CreateProfile",
	CmdUpdateProfile:                    "UpdateProfile",
	CmdDeleteProfile:                    "DeleteProfile",
	CmdCreateAPIToken:                   "CreateAPIToken",
	CmdDeleteAPIToken:                   "DeleteAPIToken",
	CmdCreateSession:                    "CreateSession",
	CmdDeleteSessionByTokenHash:         "DeleteSessionByTokenHash",
	CmdDeleteExpiredSessions:            "DeleteExpiredSessions",
	CmdDeleteOldestSessions:             "DeleteOldestSessions",
	CmdDeleteAllSessionsForUser:         "DeleteAllSessionsForUser",
	CmdDeleteAllSessions:                "DeleteAllSessions",
	CmdCreateNetworkSlice:               "CreateNetworkSlice",
	CmdUpdateNetworkSlice:               "UpdateNetworkSlice",
	CmdDeleteNetworkSlice:               "DeleteNetworkSlice",
	CmdCreateDataNetwork:                "CreateDataNetwork",
	CmdUpdateDataNetwork:                "UpdateDataNetwork",
	CmdDeleteDataNetwork:                "DeleteDataNetwork",
	CmdCreatePolicy:                     "CreatePolicy",
	CmdUpdatePolicy:                     "UpdatePolicy",
	CmdDeletePolicy:                     "DeletePolicy",
	CmdCreateNetworkRule:                "CreateNetworkRule",
	CmdUpdateNetworkRule:                "UpdateNetworkRule",
	CmdDeleteNetworkRule:                "DeleteNetworkRule",
	CmdDeleteNetworkRulesByPolicy:       "DeleteNetworkRulesByPolicy",
	CmdCreateHomeNetworkKey:             "CreateHomeNetworkKey",
	CmdDeleteHomeNetworkKey:             "DeleteHomeNetworkKey",
	CmdCreateBGPPeer:                    "CreateBGPPeer",
	CmdUpdateBGPPeer:                    "UpdateBGPPeer",
	CmdDeleteBGPPeer:                    "DeleteBGPPeer",
	CmdUpdateBGPSettings:                "UpdateBGPSettings",
	CmdSetImportPrefixesForPeer:         "SetImportPrefixesForPeer",
	CmdUpdateNATSettings:                "UpdateNATSettings",
	CmdUpdateN3Settings:                 "UpdateN3Settings",
	CmdUpdateFlowAccountingSettings:     "UpdateFlowAccountingSettings",
	CmdSetRetentionPolicy:               "SetRetentionPolicy",
	CmdInitializeOperator:               "InitializeOperator",
	CmdUpdateOperatorTracking:           "UpdateOperatorTracking",
	CmdUpdateOperatorID:                 "UpdateOperatorID",
	CmdUpdateOperatorCode:               "UpdateOperatorCode",
	CmdUpdateOperatorSecurityAlgorithms: "UpdateOperatorSecurityAlgorithms",
	CmdUpdateOperatorSPN:                "UpdateOperatorSPN",
	CmdUpdateOperatorAMFIdentity:        "UpdateOperatorAMFIdentity",
	CmdUpdateOperatorClusterID:          "UpdateOperatorClusterID",
	CmdSetJWTSecret:                     "SetJWTSecret",
	CmdCreateRoute:                      "CreateRoute",
	CmdDeleteRoute:                      "DeleteRoute",
	CmdUpsertClusterMember:              "UpsertClusterMember",
	CmdDeleteClusterMember:              "DeleteClusterMember",
	CmdMigrateShared:                    "MigrateShared",
	CmdRestore:                          "Restore",
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
