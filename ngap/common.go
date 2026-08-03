// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package ngap encodes and decodes NG Application Protocol messages
// (3GPP TS 38.413) using Aligned PER. It is a pure codec: no transport, no
// state, no procedure logic.
//
// Each message type's Marshal returns a complete NGAP-PDU; the matching
// ParseXxx takes the open-type payload of one.
//
// # presence
//
// A field is a value type only where its absence stops the message from
// reaching the caller, which TS 38.413 §10.3.5 makes true exactly for
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
//     decode, which §10.3.5 and §8.4.4.2 need to address the response.
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
// Conditional IEs are modeled where TS 38.413 marks them conditional, with the
// condition stated on the IE table row. None of the messages modeled here has
// one.
//
// # CHOICEs
//
// Where S1AP closes a CHOICE with an extension marker, NGAP closes it with a
// choice-Extensions alternative holding an open IE container (§9.3). Those
// CHOICEs are hand-coded, and an extension alternative this version does not
// model is an explicit error rather than a silently skipped value.
package ngap

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
// TS 38.413 §10.3.1 case 6 makes an abstract syntax error handled on the
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

// ProcedureCode ::= INTEGER (0..255). The named values are the NGAP elementary
// procedure codes (TS 38.413, NGAP-Constants).
type ProcedureCode uint8

const (
	ProcAMFConfigurationUpdate                ProcedureCode = 0
	ProcAMFStatusIndication                   ProcedureCode = 1
	ProcCellTrafficTrace                      ProcedureCode = 2
	ProcDeactivateTrace                       ProcedureCode = 3
	ProcDownlinkNASTransport                  ProcedureCode = 4
	ProcDownlinkNonUEAssociatedNRPPaTransport ProcedureCode = 5
	ProcDownlinkRANConfigurationTransfer      ProcedureCode = 6
	ProcDownlinkRANStatusTransfer             ProcedureCode = 7
	ProcDownlinkUEAssociatedNRPPaTransport    ProcedureCode = 8
	ProcErrorIndication                       ProcedureCode = 9
	ProcHandoverCancel                        ProcedureCode = 10
	ProcHandoverNotification                  ProcedureCode = 11
	ProcHandoverPreparation                   ProcedureCode = 12
	ProcHandoverResourceAllocation            ProcedureCode = 13
	ProcInitialContextSetup                   ProcedureCode = 14
	ProcInitialUEMessage                      ProcedureCode = 15
	ProcLocationReportingControl              ProcedureCode = 16
	ProcLocationReportingFailureIndication    ProcedureCode = 17
	ProcLocationReport                        ProcedureCode = 18
	ProcNASNonDeliveryIndication              ProcedureCode = 19
	ProcNGReset                               ProcedureCode = 20
	ProcNGSetup                               ProcedureCode = 21
	ProcOverloadStart                         ProcedureCode = 22
	ProcOverloadStop                          ProcedureCode = 23
	ProcPaging                                ProcedureCode = 24
	ProcPathSwitchRequest                     ProcedureCode = 25
	ProcPDUSessionResourceModify              ProcedureCode = 26
	ProcPDUSessionResourceModifyIndication    ProcedureCode = 27
	ProcPDUSessionResourceRelease             ProcedureCode = 28
	ProcPDUSessionResourceSetup               ProcedureCode = 29
	ProcPDUSessionResourceNotify              ProcedureCode = 30
	ProcPrivateMessage                        ProcedureCode = 31
	ProcPWSCancel                             ProcedureCode = 32
	ProcPWSFailureIndication                  ProcedureCode = 33
	ProcPWSRestartIndication                  ProcedureCode = 34
	ProcRANConfigurationUpdate                ProcedureCode = 35
	ProcRerouteNASRequest                     ProcedureCode = 36
	ProcRRCInactiveTransitionReport           ProcedureCode = 37
	ProcTraceFailureIndication                ProcedureCode = 38
	ProcTraceStart                            ProcedureCode = 39
	ProcUEContextModification                 ProcedureCode = 40
	ProcUEContextRelease                      ProcedureCode = 41
	ProcUEContextReleaseRequest               ProcedureCode = 42
	ProcUERadioCapabilityCheck                ProcedureCode = 43
	ProcUERadioCapabilityInfoIndication       ProcedureCode = 44
	ProcUETNLABindingRelease                  ProcedureCode = 45
	ProcUplinkNASTransport                    ProcedureCode = 46
	ProcUplinkNonUEAssociatedNRPPaTransport   ProcedureCode = 47
	ProcUplinkRANConfigurationTransfer        ProcedureCode = 48
	ProcUplinkRANStatusTransfer               ProcedureCode = 49
	ProcUplinkUEAssociatedNRPPaTransport      ProcedureCode = 50
	ProcWriteReplaceWarning                   ProcedureCode = 51
	ProcSecondaryRATDataUsageReport           ProcedureCode = 52
	ProcUplinkRIMInformationTransfer          ProcedureCode = 53
	ProcDownlinkRIMInformationTransfer        ProcedureCode = 54
	ProcRetrieveUEInformation                 ProcedureCode = 55
	ProcUEInformationTransfer                 ProcedureCode = 56
	ProcRANCPRelocationIndication             ProcedureCode = 57
	ProcUEContextResume                       ProcedureCode = 58
	ProcUEContextSuspend                      ProcedureCode = 59
	ProcUERadioCapabilityIDMapping            ProcedureCode = 60
	ProcHandoverSuccess                       ProcedureCode = 61
	ProcUplinkRANEarlyStatusTransfer          ProcedureCode = 62
	ProcDownlinkRANEarlyStatusTransfer        ProcedureCode = 63
	ProcAMFCPRelocationIndication             ProcedureCode = 64
	ProcConnectionEstablishmentIndication     ProcedureCode = 65
	ProcBroadcastSessionModification          ProcedureCode = 66
	ProcBroadcastSessionRelease               ProcedureCode = 67
	ProcBroadcastSessionSetup                 ProcedureCode = 68
	ProcDistributionSetup                     ProcedureCode = 69
	ProcDistributionRelease                   ProcedureCode = 70
	ProcMulticastSessionActivation            ProcedureCode = 71
	ProcMulticastSessionDeactivation          ProcedureCode = 72
	ProcMulticastSessionUpdate                ProcedureCode = 73
	ProcMulticastGroupPaging                  ProcedureCode = 74
	ProcBroadcastSessionReleaseRequired       ProcedureCode = 75
	ProcTimingSynchronisationStatus           ProcedureCode = 76
	ProcTimingSynchronisationStatusReport     ProcedureCode = 77
	ProcMTCommunicationHandling               ProcedureCode = 78
	ProcRANPagingRequest                      ProcedureCode = 79
	ProcBroadcastSessionTransport             ProcedureCode = 80
)
