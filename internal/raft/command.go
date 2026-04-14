// Copyright 2026 Ella Networks

package raft

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

// CommandType identifies a shared-DB write operation in the Raft log.
// Each type maps to exactly one applyX function in the FSM.
type CommandType uint16

const (
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

// ErrCommandNotYetAvailable is returned by Propose when a command's
// introduction version exceeds the cluster-wide minimum protocol (CWMP).
var ErrCommandNotYetAvailable = errors.New("command not yet available at current cluster-wide minimum protocol version")

// commandNames provides human-readable names for logging and debugging.
var commandNames = map[CommandType]string{
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

// commandIntroducedAt maps each CommandType to the protocol version (semver
// minor) in which it was first available. The Propose CWMP gate uses this to
// prevent proposing commands that older voters cannot apply.
var commandIntroducedAt = map[CommandType]int{
	CmdCreateSubscriber:                 9,
	CmdUpdateSubscriberProfile:          9,
	CmdEditSubscriberSeqNum:             9,
	CmdDeleteSubscriber:                 9,
	CmdIncrementDailyUsage:              9,
	CmdClearDailyUsage:                  9,
	CmdDeleteOldDailyUsage:              9,
	CmdCreateLease:                      9,
	CmdUpdateLeaseSession:               9,
	CmdDeleteDynamicLease:               9,
	CmdDeleteAllDynamicLeases:           9,
	CmdDeleteDynamicLeasesByNode:        9,
	CmdUpdateLeaseNode:                  9,
	CmdInsertAuditLog:                   9,
	CmdDeleteOldAuditLogs:               9,
	CmdCreateUser:                       9,
	CmdUpdateUser:                       9,
	CmdUpdateUserPassword:               9,
	CmdDeleteUser:                       9,
	CmdCreateProfile:                    9,
	CmdUpdateProfile:                    9,
	CmdDeleteProfile:                    9,
	CmdCreateAPIToken:                   9,
	CmdDeleteAPIToken:                   9,
	CmdCreateSession:                    9,
	CmdDeleteSessionByTokenHash:         9,
	CmdDeleteExpiredSessions:            9,
	CmdDeleteOldestSessions:             9,
	CmdDeleteAllSessionsForUser:         9,
	CmdDeleteAllSessions:                9,
	CmdCreateNetworkSlice:               9,
	CmdUpdateNetworkSlice:               9,
	CmdDeleteNetworkSlice:               9,
	CmdCreateDataNetwork:                9,
	CmdUpdateDataNetwork:                9,
	CmdDeleteDataNetwork:                9,
	CmdCreatePolicy:                     9,
	CmdUpdatePolicy:                     9,
	CmdDeletePolicy:                     9,
	CmdCreateNetworkRule:                9,
	CmdUpdateNetworkRule:                9,
	CmdDeleteNetworkRule:                9,
	CmdDeleteNetworkRulesByPolicy:       9,
	CmdCreateHomeNetworkKey:             9,
	CmdDeleteHomeNetworkKey:             9,
	CmdCreateBGPPeer:                    9,
	CmdUpdateBGPPeer:                    9,
	CmdDeleteBGPPeer:                    9,
	CmdUpdateBGPSettings:                9,
	CmdSetImportPrefixesForPeer:         9,
	CmdUpdateNATSettings:                9,
	CmdUpdateN3Settings:                 9,
	CmdUpdateFlowAccountingSettings:     9,
	CmdSetRetentionPolicy:               9,
	CmdInitializeOperator:               9,
	CmdUpdateOperatorTracking:           9,
	CmdUpdateOperatorID:                 9,
	CmdUpdateOperatorCode:               9,
	CmdUpdateOperatorSecurityAlgorithms: 9,
	CmdUpdateOperatorSPN:                9,
	CmdUpdateOperatorAMFIdentity:        9,
	CmdUpdateOperatorClusterID:          9,
	CmdSetJWTSecret:                     9,
	CmdCreateRoute:                      9,
	CmdDeleteRoute:                      9,
	CmdUpsertClusterMember:              9,
	CmdDeleteClusterMember:              9,
	CmdMigrateShared:                    9,
	CmdRestore:                          9,
}

// Introduced returns the protocol version in which the given CommandType was
// first available. Panics if the command is missing from the introduction
// table — every constant must have an entry.
func Introduced(cmd CommandType) int {
	v, ok := commandIntroducedAt[cmd]
	if !ok {
		panic(fmt.Sprintf("CommandType %d missing from introduction table", cmd))
	}

	return v
}
