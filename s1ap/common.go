// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package s1ap encodes and decodes S1 Application Protocol messages
// (3GPP TS 36.413) using Aligned PER. It is a pure codec: no transport, no
// state, no procedure logic.
//
// Each message type's Marshal returns a complete S1AP-PDU; the matching
// ParseXxx takes the open-type payload of one.
//
// # presence
//
// A field is a value type only where its absence stops the message from
// reaching the caller, which TS 36.413 §10.3.5 makes true exactly for
// required IEs of reject criticality. Every other IE is nil-able — a pointer,
// or a nil slice where the type is already a slice — so an absent IE is never
// confused with a zero one. Because criticality is assigned per message, the
// same IE can be a pointer in one message and a value in another.
//
// Encoding holds the sender to the stricter rule of §10.3.3: Marshal fails if
// an IE the message must carry is unset, whatever its criticality.
//
// # Errors
//
// Decoding reports one of two errors, and returns no message with either:
//
//   - [TransferSyntaxError], for octets that are not decodable.
//   - [AbstractSyntaxError], for a message whose procedure must be rejected —
//     a missing reject-criticality IE (§10.3.5), an unhandled reject-criticality
//     IE (§10.3.4.2), or a falsely constructed message (§10.3.6). It carries
//     the cause to report, per-IE Criticality Diagnostics, and the IEs that did
//     decode, which §10.3.5 and §8.7.2.2 need to address the response.
//
// Everything §10.3.4.2 and §10.3.5 let a receiver carry on past — an absent
// ignore or notify IE, an IE this version does not model — is not an error.
// The message is returned and the detail is reported by [Diagnostics], which
// also builds the Criticality Diagnostics to answer with.
//
// Unmodeled IEs are preserved and re-emitted on encode; see [UnknownIEs].
// Preservation and diagnostics are both bounded, since a peer chooses how many
// IEs to send; [Diagnostics.Truncated] reports when a bound was reached.
//
// Conditional IEs are modeled where TS 36.413 marks them conditional, with the
// condition stated on the IE table row. None of the messages modeled here has
// one.
package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// Ptr returns a pointer to v, for setting an optional IE inline.
func Ptr[T any](v T) *T { return &v }

// Criticality ::= ENUMERATED { reject, ignore, notify } (not extensible).
type Criticality uint8

const (
	CriticalityReject Criticality = iota
	CriticalityIgnore
	CriticalityNotify

	criticalityRootCount = 3
)

func (c Criticality) String() string {
	switch c {
	case CriticalityReject:
		return "reject"
	case CriticalityIgnore:
		return "ignore"
	case CriticalityNotify:
		return "notify"
	default:
		return fmt.Sprintf("Criticality(%d)", uint8(c))
	}
}

// decodeRootEnumerated decodes an extensible ENUMERATED and refuses an
// extension addition.
//
// per.DecodeEnumerated reports the k'th extension value as nRoot+k, which is
// what keeps it distinct from a root value — but every enumeration here is a
// small unsigned Go type, so storing nRoot+k narrows it straight back onto a
// root value. A peer's unknown future value would then read as a specific one
// this version does know, silently and with no error.
//
// An unknown value is a functionality this version does not support, which
// TS 36.413 §10.3.1 case 6 makes an abstract syntax error handled on the
// IE's criticality rather than a transfer syntax error.
func decodeRootEnumerated(r *per.Reader, enc per.Encoding, nRoot int64, name string) (int64, error) {
	idx, err := per.DecodeEnumerated(r, enc, nRoot, true)
	if err != nil {
		return 0, err
	}

	if idx >= nRoot {
		return 0, fmt.Errorf("%w: %s extension value %d", errNotComprehended, name, idx)
	}

	return idx, nil
}

// presence ::= ENUMERATED { optional, conditional, mandatory }. It drives the
// IE-container engine's handling of a missing IE.
type presence uint8

const (
	presenceOptional presence = iota
	presenceConditional
	presenceMandatory
)

// TriggeringMessage ::= ENUMERATED { initiating-message, successful-outcome,
// unsuccessful-outcome } (used by CriticalityDiagnostics).
type TriggeringMessage uint8

const (
	TriggeringInitiatingMessage TriggeringMessage = iota
	TriggeringSuccessfulOutcome
	TriggeringUnsuccessfulOutcome

	triggeringMessageRootCount = 3
)

// ProtocolIEID ::= INTEGER (0..65535).
type ProtocolIEID uint16

// ProcedureCode ::= INTEGER (0..255). The named values are the S1AP elementary
// procedure codes (TS 36.413, S1AP-Constants).
type ProcedureCode uint8

const (
	ProcHandoverPreparation                ProcedureCode = 0
	ProcHandoverResourceAllocation         ProcedureCode = 1
	ProcHandoverNotification               ProcedureCode = 2
	ProcPathSwitchRequest                  ProcedureCode = 3
	ProcHandoverCancel                     ProcedureCode = 4
	ProcERABSetup                          ProcedureCode = 5
	ProcERABModify                         ProcedureCode = 6
	ProcERABRelease                        ProcedureCode = 7
	ProcERABReleaseIndication              ProcedureCode = 8
	ProcInitialContextSetup                ProcedureCode = 9
	ProcPaging                             ProcedureCode = 10
	ProcDownlinkNASTransport               ProcedureCode = 11
	ProcInitialUEMessage                   ProcedureCode = 12
	ProcUplinkNASTransport                 ProcedureCode = 13
	ProcReset                              ProcedureCode = 14
	ProcErrorIndication                    ProcedureCode = 15
	ProcNASNonDeliveryIndication           ProcedureCode = 16
	ProcS1Setup                            ProcedureCode = 17
	ProcUEContextReleaseRequest            ProcedureCode = 18
	ProcDownlinkS1cdma2000tunnelling       ProcedureCode = 19
	ProcUplinkS1cdma2000tunnelling         ProcedureCode = 20
	ProcUEContextModification              ProcedureCode = 21
	ProcUECapabilityInfoIndication         ProcedureCode = 22
	ProcUEContextRelease                   ProcedureCode = 23
	ProcENBStatusTransfer                  ProcedureCode = 24
	ProcMMEStatusTransfer                  ProcedureCode = 25
	ProcDeactivateTrace                    ProcedureCode = 26
	ProcTraceStart                         ProcedureCode = 27
	ProcTraceFailureIndication             ProcedureCode = 28
	ProcENBConfigurationUpdate             ProcedureCode = 29
	ProcMMEConfigurationUpdate             ProcedureCode = 30
	ProcLocationReportingControl           ProcedureCode = 31
	ProcLocationReportingFailureIndication ProcedureCode = 32
	ProcLocationReport                     ProcedureCode = 33
	ProcOverloadStart                      ProcedureCode = 34
	ProcOverloadStop                       ProcedureCode = 35
	ProcWriteReplaceWarning                ProcedureCode = 36
	ProcENBDirectInformationTransfer       ProcedureCode = 37
	ProcMMEDirectInformationTransfer       ProcedureCode = 38
	ProcPrivateMessage                     ProcedureCode = 39
	ProcENBConfigurationTransfer           ProcedureCode = 40
	ProcMMEConfigurationTransfer           ProcedureCode = 41
	ProcCellTrafficTrace                   ProcedureCode = 42
	ProcKill                               ProcedureCode = 43
	ProcDownlinkUEAssociatedLPPaTransport  ProcedureCode = 44
	ProcUplinkUEAssociatedLPPaTransport    ProcedureCode = 45
	ProcDownlinkNonUEAssociatedLPPa        ProcedureCode = 46
	ProcUplinkNonUEAssociatedLPPa          ProcedureCode = 47
	ProcUERadioCapabilityMatch             ProcedureCode = 48
	ProcPWSRestartIndication               ProcedureCode = 49
	ProcERABModificationIndication         ProcedureCode = 50
	ProcPWSFailureIndication               ProcedureCode = 51
	ProcRerouteNASRequest                  ProcedureCode = 52
	ProcUEContextModificationIndication    ProcedureCode = 53
	ProcConnectionEstablishmentIndication  ProcedureCode = 54
	ProcUEContextSuspend                   ProcedureCode = 55
	ProcUEContextResume                    ProcedureCode = 56
	ProcNASDeliveryIndication              ProcedureCode = 57
	ProcRetrieveUEInformation              ProcedureCode = 58
	ProcUEInformationTransfer              ProcedureCode = 59
	ProcENBCPRelocationIndication          ProcedureCode = 60
	ProcMMECPRelocationIndication          ProcedureCode = 61
	ProcSecondaryRATDataUsageReport        ProcedureCode = 62
	ProcUERadioCapabilityIDMapping         ProcedureCode = 63
	ProcHandoverSuccess                    ProcedureCode = 64
	ProcENBEarlyStatusTransfer             ProcedureCode = 65
	ProcMMEEarlyStatusTransfer             ProcedureCode = 66
)
